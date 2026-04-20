package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"s3server/auth"
	"s3server/config"
	"s3server/logger"
	"s3server/storage"
)

type Router struct {
	storage   *storage.FileStorage
	config    *config.Config
	bucket    *BucketHandler
	object    *ObjectHandler
	multipart *MultipartHandler
}

func NewRouter(store *storage.FileStorage, cfg *config.Config) *Router {
	return &Router{
		storage:   store,
		config:    cfg,
		bucket:    NewBucketHandler(store),
		object:    NewObjectHandler(store),
		multipart: NewMultipartHandler(store),
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger.Debug("%s %s", req.Method, req.URL.Path)

	path := strings.TrimPrefix(req.URL.Path, "/")

	if path == "" {
		r.listBuckets(w, req)
		return
	}

	parts := strings.SplitN(path, "/", 2)
	bucket := parts[0]

	if !r.storage.BucketExists(bucket) {
		if len(parts) > 1 {
			r.bucket.handle(w, req)
		} else {
			r.listBuckets(w, req)
		}
		return
	}

	switch req.Method {
	case "GET":
		if len(parts) == 1 {
			r.listObjects(w, req)
		} else {
			r.object.handle(w, req)
		}
	case "PUT":
		r.object.handle(w, req)
	case "HEAD":
		if len(parts) == 1 {
			r.listObjects(w, req)
		} else {
			r.object.head(w, req)
		}
	case "DELETE":
		if len(parts) == 1 {
			r.bucket.handle(w, req)
		} else {
			r.object.handle(w, req)
		}
	case "POST":
		r.multipart.handle(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Router) listBuckets(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" && req.Method != "HEAD" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.bucket.list(w, req)
}

func (r *Router) listObjects(w http.ResponseWriter, req *http.Request) {
	r.bucket.listObjects(w, req)
}

func validateAuth(req *http.Request) error {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("AccessDenied: Missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "AWS4-HMAC-SHA256") {
		return fmt.Errorf("AccessDenied: Invalid Authorization header")
	}

	validator := auth.NewSignatureValidator(req)
	_, err := validator.Validate(authHeader)
	return err
}
