use crate::config::AppConfig;
use md5::Digest;
use std::fs::{self, File};
use std::io::{Read, Write};
use std::path::PathBuf;
use std::time::SystemTime;

#[derive(Clone)]
pub struct Storage {
    data_dir: PathBuf,
}

impl Storage {
    pub fn new(config: &AppConfig) -> Self {
        Storage {
            data_dir: config.data_dir.clone(),
        }
    }

    fn bucket_path(&self, bucket: &str) -> PathBuf {
        self.data_dir.join(bucket)
    }

    fn object_path(&self, bucket: &str, key: &str) -> PathBuf {
        self.data_dir.join(bucket).join(key)
    }

    pub fn list_buckets(&self) -> Vec<String> {
        let mut buckets = Vec::new();
        if let Ok(entries) = fs::read_dir(&self.data_dir) {
            for entry in entries.flatten() {
                if entry.path().is_dir() {
                    if let Some(name) = entry.file_name().to_str() {
                        if !name.starts_with('.') {
                            buckets.push(name.to_string());
                        }
                    }
                }
            }
        }
        buckets.sort();
        buckets
    }

    pub fn create_bucket(&self, bucket: &str) -> Result<(), String> {
        let path = self.bucket_path(bucket);
        fs::create_dir_all(&path).map_err(|e| e.to_string())
    }

    pub fn bucket_exists(&self, bucket: &str) -> bool {
        self.bucket_path(bucket).is_dir()
    }

    pub fn delete_bucket(&self, bucket: &str) -> Result<(), String> {
        let path = self.bucket_path(bucket);
        if path.is_dir() {
            fs::remove_dir_all(&path).map_err(|e| e.to_string())?;
        }
        Ok(())
    }

    pub fn bucket_empty(&self, bucket: &str) -> bool {
        if let Ok(entries) = fs::read_dir(self.bucket_path(bucket)) {
            entries.count() == 0
        } else {
            true
        }
    }

    pub fn put_object(&self, bucket: &str, key: &str, data: Vec<u8>) -> Result<String, String> {
        validate_key(key)?;

        // Check file size (10GB max)
        if data.len() > 10 * 1024 * 1024 * 1024 {
            return Err("File size exceeds maximum allowed".to_string());
        }

        let path = self.object_path(bucket, key);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }

        let mut file = File::create(&path).map_err(|e| e.to_string())?;
        file.write_all(&data).map_err(|e| e.to_string())?;

        Ok(calculate_etag(&path))
    }

    pub fn get_object(&self, bucket: &str, key: &str) -> Result<Vec<u8>, String> {
        validate_key(key)?;
        let path = self.object_path(bucket, key);
        let mut file = File::open(&path).map_err(|e| e.to_string())?;
        let mut data = Vec::new();
        file.read_to_end(&mut data).map_err(|e| e.to_string())?;
        Ok(data)
    }

    pub fn delete_object(&self, bucket: &str, key: &str) -> Result<(), String> {
        validate_key(key)?;
        let path = self.object_path(bucket, key);
        fs::remove_file(&path).map_err(|e| e.to_string())
    }

    pub fn object_exists(&self, bucket: &str, key: &str) -> bool {
        self.object_path(bucket, key).is_file()
    }

    pub fn get_object_meta(&self, bucket: &str, key: &str) -> Result<ObjectMeta, String> {
        validate_key(key)?;
        let path = self.object_path(bucket, key);
        let metadata = fs::metadata(&path).map_err(|e| e.to_string())?;

        Ok(ObjectMeta {
            key: key.to_string(),
            size: metadata.len(),
            last_modified: metadata.modified().ok(),
            etag: calculate_etag(&path),
            mime_type: guess_mime_type(key),
        })
    }

    pub fn list_objects(
        &self,
        bucket: &str,
        prefix: &str,
        max_keys: usize,
    ) -> (Vec<ObjectMeta>, bool) {
        let mut objects = Vec::new();
        let bucket_path = self.bucket_path(bucket);

        if let Ok(entries) = fs::read_dir(&bucket_path) {
            for entry in entries.flatten() {
                if entry.path().is_file() {
                    if let Some(name) = entry.file_name().to_str() {
                        if name.starts_with(prefix) {
                            if let Ok(meta) = self.get_object_meta(bucket, name) {
                                objects.push(meta);
                            }
                        }
                    }
                }
            }
        }

        objects.sort_by(|a, b| a.key.cmp(&b.key));

        let is_truncated = objects.len() > max_keys;
        if is_truncated {
            objects.truncate(max_keys);
        }

        (objects, is_truncated)
    }

    pub fn copy_object(
        &self,
        src_bucket: &str,
        src_key: &str,
        dest_bucket: &str,
        dest_key: &str,
    ) -> Result<String, String> {
        let data = self.get_object(src_bucket, src_key)?;
        self.put_object(dest_bucket, dest_key, data)
    }

    pub fn create_multipart_upload(&self, bucket: &str, upload_id: &str) -> Result<(), String> {
        let path = self
            .data_dir
            .join(".multiparts")
            .join(bucket)
            .join(upload_id);
        fs::create_dir_all(&path).map_err(|e| e.to_string())
    }

    pub fn upload_exists(&self, bucket: &str, upload_id: &str) -> bool {
        self.data_dir
            .join(".multiparts")
            .join(bucket)
            .join(upload_id)
            .is_dir()
    }

    pub fn save_part(
        &self,
        bucket: &str,
        upload_id: &str,
        part_number: u32,
        data: Vec<u8>,
    ) -> Result<String, String> {
        let path = self
            .data_dir
            .join(".multiparts")
            .join(bucket)
            .join(upload_id)
            .join(format!("part-{}", part_number));
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).map_err(|e| e.to_string())?;
        }
        let mut file = File::create(&path).map_err(|e| e.to_string())?;
        file.write_all(&data).map_err(|e| e.to_string())?;
        Ok(calculate_etag(&path))
    }

    pub fn get_part(
        &self,
        bucket: &str,
        upload_id: &str,
        part_number: u32,
    ) -> Result<Vec<u8>, String> {
        let path = self
            .data_dir
            .join(".multiparts")
            .join(bucket)
            .join(upload_id)
            .join(format!("part-{}", part_number));
        let mut file = File::open(&path).map_err(|e| e.to_string())?;
        let mut data = Vec::new();
        file.read_to_end(&mut data).map_err(|e| e.to_string())?;
        Ok(data)
    }

    pub fn complete_multipart_upload(
        &self,
        bucket: &str,
        key: &str,
        upload_id: &str,
        parts: &Vec<Vec<u8>>,
    ) -> Result<CompleteResult, String> {
        let mut data = Vec::new();
        for part_data in parts {
            data.extend_from_slice(part_data);
        }

        let etag = self.put_object(bucket, key, data.clone())?;

        // Cleanup multipart
        let path = self
            .data_dir
            .join(".multiparts")
            .join(bucket)
            .join(upload_id);
        fs::remove_dir_all(&path).ok();

        Ok(CompleteResult {
            etag,
            size: data.len() as u64,
        })
    }

    pub fn abort_multipart_upload(&self, bucket: &str, upload_id: &str) -> Result<(), String> {
        let path = self
            .data_dir
            .join(".multiparts")
            .join(bucket)
            .join(upload_id);
        fs::remove_dir_all(&path).map_err(|e| e.to_string())
    }
}

fn validate_key(key: &str) -> Result<(), String> {
    if key.contains("..") {
        return Err("Path traversal not allowed".to_string());
    }
    if key.contains('\\') {
        return Err("Backslash not allowed in key".to_string());
    }
    Ok(())
}

fn calculate_etag(path: &PathBuf) -> String {
    if let Ok(mut file) = File::open(path) {
        let mut hasher = md5::Md5::new();
        let mut buffer = Vec::new();
        if file.read_to_end(&mut buffer).is_ok() {
            hasher.update(&buffer);
            return format!("{:x}", hasher.finalize());
        }
    }
    String::from("0")
}

fn guess_mime_type(key: &str) -> String {
    let ext = key.rsplit('.').next().unwrap_or("");
    match ext.to_lowercase().as_str() {
        "txt" => "text/plain",
        "html" | "htm" => "text/html",
        "css" => "text/css",
        "js" => "application/javascript",
        "json" => "application/json",
        "xml" => "application/xml",
        "pdf" => "application/pdf",
        "zip" => "application/zip",
        "jpg" | "jpeg" => "image/jpeg",
        "png" => "image/png",
        "gif" => "image/gif",
        "svg" => "image/svg+xml",
        _ => "application/octet-stream",
    }
    .to_string()
}

#[derive(Clone)]
pub struct ObjectMeta {
    pub key: String,
    pub size: u64,
    pub last_modified: Option<SystemTime>,
    pub etag: String,
    pub mime_type: String,
}

pub struct CompleteResult {
    pub etag: String,
    pub size: u64,
}
