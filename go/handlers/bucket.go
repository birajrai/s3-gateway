package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"s3server/logger"
	"s3server/storage"
	"s3server/xmlresp"
)

type BucketHandler struct {
	storage *storage.FileStorage
}

func NewBucketHandler(store *storage.FileStorage) *BucketHandler {
	return &BucketHandler{storage: store}
}

func (h *BucketHandler) handle(w http.ResponseWriter, req *http.Request) {
	if err := validateAuth(req); err != nil {
		ErrorResponse(w, "AccessDenied", err.Error())
		return
	}

	switch req.Method {
	case "GET":
		h.list(w, req)
	case "PUT":
		h.create(w, req)
	case "DELETE":
		h.delete(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BucketHandler) list(w http.ResponseWriter, req *http.Request) {
	buckets, err := h.storage.ListBuckets()
	if err != nil {
		logger.Error("Failed to list buckets: %v", err)
		ErrorResponse(w, "InternalError", "Failed to list buckets")
		return
	}

	dataDir := h.storage.DataDir()
	xmlResp := xmlresp.ListBuckets(buckets, dataDir)

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xmlResp))
}

func (h *BucketHandler) create(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	bucket := strings.SplitN(path, "/", 2)[0]

	if bucket == "" {
		ErrorResponse(w, "InvalidBucketName", "Bucket name required")
		return
	}

	if err := h.storage.CreateBucket(bucket); err != nil {
		logger.Error("Failed to create bucket: %v", err)
		ErrorResponse(w, "InternalError", "Failed to create bucket")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *BucketHandler) delete(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	bucket := strings.SplitN(path, "/", 2)[0]

	if bucket == "" {
		ErrorResponse(w, "InvalidBucketName", "Bucket name required")
		return
	}

	if !h.storage.BucketExists(bucket) {
		ErrorResponse(w, "NoSuchBucket", "Bucket does not exist")
		return
	}

	if !h.storage.BucketEmpty(bucket) {
		ErrorResponse(w, "BucketNotEmpty", "Bucket is not empty")
		return
	}

	if err := h.storage.DeleteBucket(bucket); err != nil {
		logger.Error("Failed to delete bucket: %v", err)
		ErrorResponse(w, "InternalError", "Failed to delete bucket")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *BucketHandler) listObjects(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]

	if bucket == "" {
		ErrorResponse(w, "InvalidBucketName", "Bucket name required")
		return
	}

	if !h.storage.BucketExists(bucket) {
		ErrorResponse(w, "NoSuchBucket", "Bucket does not exist")
		return
	}

	prefix := req.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = ""
	}

	maxKeys := 1000
	if maxKeysStr := req.URL.Query().Get("max-keys"); maxKeysStr != "" {
		fmt.Sscanf(maxKeysStr, "%d", &maxKeys)
	}

	skip := 0
	if contToken := req.URL.Query().Get("continuation-token"); contToken != "" {
		fmt.Sscanf(contToken, "%d", &skip)
	}

	objects, isTruncated, err := h.storage.ListObjects(bucket, prefix, maxKeys, skip)
	if err != nil {
		logger.Error("Failed to list objects: %v", err)
		ErrorResponse(w, "InternalError", "Failed to list objects")
		return
	}

	startAfter := req.URL.Query().Get("start-after")
	if startAfter == "" {
		startAfter = ""
	}

	xmlResp := xmlresp.ListObjectsV2(objects, bucket, prefix, maxKeys, isTruncated, startAfter)

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xmlResp))
}
