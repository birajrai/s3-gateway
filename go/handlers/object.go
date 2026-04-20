package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"s3server/logger"
	"s3server/storage"
)

type ObjectHandler struct {
	storage *storage.FileStorage
}

func NewObjectHandler(store *storage.FileStorage) *ObjectHandler {
	return &ObjectHandler{storage: store}
}

func (h *ObjectHandler) handle(w http.ResponseWriter, req *http.Request) {
	if err := validateAuth(req); err != nil {
		ErrorResponse(w, "AccessDenied", err.Error())
		return
	}

	switch req.Method {
	case "GET":
		h.get(w, req)
	case "PUT":
		h.put(w, req)
	case "DELETE":
		h.delete(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ObjectHandler) put(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	key := parts[1]

	if bucket == "" || key == "" {
		ErrorResponse(w, "InvalidArgument", "Bucket and key required")
		return
	}

	if !h.storage.BucketExists(bucket) {
		ErrorResponse(w, "NoSuchBucket", "Bucket does not exist")
		return
	}

	data, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("Failed to read request body: %v", err)
		ErrorResponse(w, "InternalError", "Failed to read request body")
		return
	}

	if err := h.storage.PutObject(bucket, key, data); err != nil {
		logger.Error("Failed to put object: %v", err)
		ErrorResponse(w, "InternalError", "Failed to write object")
		return
	}

	meta, err := h.storage.GetObjectMeta(bucket, key)
	if err != nil {
		logger.Error("Failed to get object meta: %v", err)
		ErrorResponse(w, "InternalError", "Failed to get object metadata")
		return
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", meta.ETag))
	w.WriteHeader(http.StatusOK)
}

func (h *ObjectHandler) get(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	key := parts[1]

	if bucket == "" || key == "" {
		ErrorResponse(w, "InvalidArgument", "Bucket and key required")
		return
	}

	if !h.storage.ObjectExists(bucket, key) {
		ErrorResponse(w, "NoSuchKey", "Object does not exist")
		return
	}

	filePath := h.storage.ObjectPath(bucket, key)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Accept-Ranges", "bytes")

	http.ServeFile(w, req, filePath)
}

func (h *ObjectHandler) head(w http.ResponseWriter, req *http.Request) {
	if err := validateAuth(req); err != nil {
		ErrorResponse(w, "AccessDenied", err.Error())
		return
	}

	path := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	key := parts[1]

	if bucket == "" || key == "" {
		ErrorResponse(w, "InvalidArgument", "Bucket and key required")
		return
	}

	meta, err := h.storage.GetObjectMeta(bucket, key)
	if err != nil {
		ErrorResponse(w, "NoSuchKey", "Object does not exist")
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))
	w.Header().Set("Content-Type", meta.MimeType)
	w.Header().Set("Last-Modified", meta.LastModified.Format("Mon, 02 Jan 2006 15:04:05 GMT"))
	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", meta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
}

func (h *ObjectHandler) delete(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	key := parts[1]

	if bucket == "" || key == "" {
		ErrorResponse(w, "InvalidArgument", "Bucket and key required")
		return
	}

	if err := h.storage.DeleteObject(bucket, key); err != nil {
		logger.Error("Failed to delete object: %v", err)
		ErrorResponse(w, "InternalError", "Failed to delete object")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
