package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"s3server/logger"
	"s3server/storage"
)

type MultipartHandler struct {
	storage *storage.FileStorage
}

func NewMultipartHandler(store *storage.FileStorage) *MultipartHandler {
	return &MultipartHandler{storage: store}
}

func (h *MultipartHandler) handle(w http.ResponseWriter, req *http.Request) {
	if err := validateAuth(req); err != nil {
		ErrorResponse(w, "AccessDenied", err.Error())
		return
	}

	path := strings.TrimPrefix(req.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]
	key := ""
	if len(parts) > 1 {
		key = parts[1]
	}

	uploadID := req.URL.Query().Get("uploadId")

	switch {
	case req.URL.Path == "/"+bucket+"/"+key+"?uploads":
		h.createMultipartUpload(w, req, bucket, key)
	case uploadID != "" && req.URL.Query().Get("partNumber") != "":
		h.uploadPart(w, req, bucket, key, uploadID)
	case uploadID != "":
		h.completeMultipartUpload(w, req, bucket, key, uploadID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *MultipartHandler) createMultipartUpload(w http.ResponseWriter, req *http.Request, bucket, key string) {
	if bucket == "" || key == "" {
		ErrorResponse(w, "InvalidArgument", "Bucket and key required")
		return
	}

	if !h.storage.BucketExists(bucket) {
		ErrorResponse(w, "NoSuchBucket", "Bucket does not exist")
		return
	}

	b := make([]byte, 16)
	rand.Read(b)
	uploadID := base64.URLEncoding.EncodeToString(b)

	if err := h.storage.CreateMultipartUpload(bucket, uploadID); err != nil {
		logger.Error("Failed to create multipart upload: %v", err)
		ErrorResponse(w, "InternalError", "Failed to create multipart upload")
		return
	}

	xmlResp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<CreateMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Bucket>%s</Bucket>
  <Key>%s</Key>
  <UploadId>%s</UploadId>
</CreateMultipartUploadResult>`, bucket, key, uploadID)

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xmlResp))
}

func (h *MultipartHandler) uploadPart(w http.ResponseWriter, req *http.Request, bucket, key, uploadID string) {
	partNumber := req.URL.Query().Get("partNumber")
	if partNumber == "" {
		ErrorResponse(w, "InvalidArgument", "partNumber required")
		return
	}

	var partNum int
	fmt.Sscanf(partNumber, "%d", &partNum)

	if !h.storage.UploadExists(bucket, uploadID) {
		ErrorResponse(w, "NoSuchUpload", "Upload does not exist")
		return
	}

	data, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("Failed to read part data: %v", err)
		ErrorResponse(w, "InternalError", "Failed to read part data")
		return
	}

	if err := h.storage.SavePart(bucket, uploadID, partNum, data); err != nil {
		logger.Error("Failed to save part: %v", err)
		ErrorResponse(w, "InternalError", "Failed to save part")
		return
	}

	meta, err := h.storage.GetPartMeta(bucket, uploadID, partNum)
	if err != nil {
		logger.Error("Failed to get part meta: %v", err)
		ErrorResponse(w, "InternalError", "Failed to get part metadata")
		return
	}

	w.Header().Set("ETag", fmt.Sprintf("\"%s\"", meta.ETag))
	w.WriteHeader(http.StatusOK)
}

func (h *MultipartHandler) completeMultipartUpload(w http.ResponseWriter, req *http.Request, bucket, key, uploadID string) {
	if !h.storage.UploadExists(bucket, uploadID) {
		ErrorResponse(w, "NoSuchUpload", "Upload does not exist")
		return
	}

	data, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("Failed to read request body: %v", err)
		ErrorResponse(w, "InternalError", "Failed to read request body")
		return
	}

	type Part struct {
		PartNumber int    `xml:"PartNumber"`
		ETag       string `xml:"ETag"`
	}

	type CompleteUpload struct {
		Parts []Part `xml:"Part"`
	}

	var upload CompleteUpload
	if err := xml.Unmarshal(data, &upload); err != nil {
		logger.Error("Failed to parse XML: %v", err)
		ErrorResponse(w, "InvalidXML", "Invalid XML")
		return
	}

	parts := make(map[int]string)
	for _, part := range upload.Parts {
		parts[part.PartNumber] = part.ETag
	}

	result, err := h.storage.CompleteMultipartUpload(bucket, key, uploadID, parts)
	if err != nil {
		logger.Error("Failed to complete multipart upload: %v", err)
		ErrorResponse(w, "InternalError", "Failed to complete multipart upload")
		return
	}

	location := fmt.Sprintf("http://%s/%s/%s", req.Host, bucket, key)
	xmlResp := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Location>%s</Location>
  <Bucket>%s</Bucket>
  <Key>%s</Key>
  <ETag>%s</ETag>
</CompleteMultipartUploadResult>`, location, bucket, key, result["etag"])

	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(xmlResp))
}
