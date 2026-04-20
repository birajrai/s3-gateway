package storage

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"s3server/config"
	"s3server/logger"
)

var maxFileSize int64 = 10 * 1024 * 1024 * 1024 // 10GB

type FileStorage struct {
	dataDir string
}

func validateKey(key string) error {
	if strings.Contains(key, "..") {
		return fmt.Errorf("path traversal not allowed")
	}
	if strings.Contains(key, "\\") {
		return fmt.Errorf("backslash not allowed in key")
	}
	return nil
}

func NewFileStorage(cfg *config.Config) (*FileStorage, error) {
	fs := &FileStorage{
		dataDir: cfg.DataDir,
	}
	return fs, nil
}

func (s *FileStorage) DataDir() string {
	return s.dataDir
}

func (s *FileStorage) bucketPath(bucket string) string {
	return filepath.Join(s.dataDir, bucket)
}

func (s *FileStorage) objectPath(bucket, key string) string {
	return filepath.Join(s.dataDir, bucket, key)
}

func (s *FileStorage) ObjectPath(bucket, key string) string {
	return s.objectPath(bucket, key)
}

func (s *FileStorage) multipartPath(bucket, uploadID string) string {
	return filepath.Join(s.dataDir, ".multiparts", bucket, uploadID)
}

func (s *FileStorage) partPath(bucket, uploadID string, partNumber int) string {
	return filepath.Join(s.multipartPath(bucket, uploadID), fmt.Sprintf("part-%d", partNumber))
}

func (s *FileStorage) ListBuckets() ([]string, error) {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, err
	}

	var buckets []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			buckets = append(buckets, entry.Name())
		}
	}
	sort.Strings(buckets)
	return buckets, nil
}

func (s *FileStorage) CreateBucket(bucket string) error {
	if strings.Contains(bucket, "..") || strings.Contains(bucket, "\\") {
		return fmt.Errorf("invalid bucket name")
	}

	path := s.bucketPath(bucket)
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	logger.Info("Created bucket: %s", bucket)
	return nil
}

func (s *FileStorage) BucketExists(bucket string) bool {
	info, err := os.Stat(s.bucketPath(bucket))
	return err == nil && info.IsDir()
}

func (s *FileStorage) DeleteBucket(bucket string) error {
	return os.RemoveAll(s.bucketPath(bucket))
}

func (s *FileStorage) BucketEmpty(bucket string) bool {
	entries, err := os.ReadDir(s.bucketPath(bucket))
	return err == nil && len(entries) == 0
}

func (s *FileStorage) PutObject(bucket, key string, data []byte) error {
	if err := validateKey(key); err != nil {
		return err
	}

	if int64(len(data)) > maxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed")
	}

	path := s.objectPath(bucket, key)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *FileStorage) GetObject(bucket, key string) ([]byte, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	return os.ReadFile(s.objectPath(bucket, key))
}

func (s *FileStorage) DeleteObject(bucket, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	return os.Remove(s.objectPath(bucket, key))
}

func (s *FileStorage) ObjectExists(bucket, key string) bool {
	if err := validateKey(key); err != nil {
		return false
	}
	_, err := os.Stat(s.objectPath(bucket, key))
	return err == nil
}

type ObjectMeta struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	MimeType     string
}

func (s *FileStorage) GetObjectMeta(bucket, key string) (*ObjectMeta, error) {
	path := s.objectPath(bucket, key)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	etag := calculateEtag(path, info.Size())

	return &ObjectMeta{
		Key:          key,
		Size:         info.Size(),
		LastModified: info.ModTime(),
		ETag:         etag,
		MimeType:     mimeType(key),
	}, nil
}

func (s *FileStorage) ListObjects(bucket, prefix string, maxKeys int, skip int) (map[string][]ObjectMeta, bool, error) {
	bucketPath := s.bucketPath(bucket)
	entries, err := os.ReadDir(bucketPath)
	if err != nil {
		return nil, false, err
	}

	var objects []ObjectMeta
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		key := entry.Name()
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		info, _ := entry.Info()
		etag := calculateEtag(filepath.Join(bucketPath, key), info.Size())

		objects = append(objects, ObjectMeta{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ETag:         etag,
			MimeType:     mimeType(key),
		})
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})

	isTruncated := false
	if len(objects) > skip+maxKeys {
		isTruncated = true
		objects = objects[skip : skip+maxKeys]
	} else if skip < len(objects) {
		objects = objects[skip:]
	}

	result := make(map[string][]ObjectMeta)
	for _, obj := range objects {
		result[obj.Key] = []ObjectMeta{obj}
	}

	return result, isTruncated, nil
}

func (s *FileStorage) CopyObject(srcBucket, srcKey, destBucket, destKey string) error {
	srcPath := s.objectPath(srcBucket, srcKey)
	destPath := s.objectPath(destBucket, destKey)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(destPath, data, 0644)
}

func (s *FileStorage) CreateMultipartUpload(bucket, uploadID string) error {
	path := s.multipartPath(bucket, uploadID)
	return os.MkdirAll(path, 0755)
}

func (s *FileStorage) UploadExists(bucket, uploadID string) bool {
	_, err := os.Stat(s.multipartPath(bucket, uploadID))
	return err == nil
}

func (s *FileStorage) SavePart(bucket, uploadID string, partNumber int, data []byte) error {
	path := s.partPath(bucket, uploadID, partNumber)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *FileStorage) GetPart(bucket, uploadID string, partNumber int) ([]byte, error) {
	return os.ReadFile(s.partPath(bucket, uploadID, partNumber))
}

type PartMeta struct {
	Number int
	Size   int64
	ETag   string
}

func (s *FileStorage) GetPartMeta(bucket, uploadID string, partNumber int) (*PartMeta, error) {
	path := s.partPath(bucket, uploadID, partNumber)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	etag := calculateEtag(path, info.Size())

	return &PartMeta{
		Number: partNumber,
		Size:   info.Size(),
		ETag:   etag,
	}, nil
}

func (s *FileStorage) ListParts(bucket, uploadID string) ([]PartMeta, error) {
	path := s.multipartPath(bucket, uploadID)
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var parts []PartMeta
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, _ := entry.Info()
		name := entry.Name()

		var partNumber int
		if _, err := fmt.Sscanf(name, "part-%d", &partNumber); err != nil {
			continue
		}

		etag := calculateEtag(filepath.Join(path, name), info.Size())

		parts = append(parts, PartMeta{
			Number: partNumber,
			Size:   info.Size(),
			ETag:   etag,
		})
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Number < parts[j].Number
	})

	return parts, nil
}

func (s *FileStorage) CompleteMultipartUpload(bucket, key, uploadID string, parts map[int]string) (map[string]interface{}, error) {
	var totalSize int64
	var data []byte

	for i := 1; i <= len(parts); i++ {
		partData, err := os.ReadFile(s.partPath(bucket, uploadID, i))
		if err != nil {
			return nil, err
		}
		data = append(data, partData...)
		totalSize += int64(len(partData))
	}

	if err := s.PutObject(bucket, key, data); err != nil {
		return nil, err
	}

	if err := s.AbortMultipartUpload(bucket, uploadID); err != nil {
		logger.Warning("Failed to cleanup multipart upload: %v", err)
	}

	etag := calculateEtag(s.objectPath(bucket, key), totalSize)

	return map[string]interface{}{
		"etag": etag,
		"size": totalSize,
	}, nil
}

func (s *FileStorage) AbortMultipartUpload(bucket, uploadID string) error {
	return os.RemoveAll(s.multipartPath(bucket, uploadID))
}

func calculateEtag(path string, size int64) string {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("%x", size)
	}
	defer f.Close()

	hash := md5.New()
	io.Copy(hash, f)

	return hex.EncodeToString(hash.Sum(nil))
}

func mimeType(key string) string {
	ext := strings.ToLower(filepath.Ext(key))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
