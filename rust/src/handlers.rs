use std::time::SystemTime;
use std::sync::Arc;
use axum::{
    extract::{State, Path, Query},
    response::Response,
    body::Body,
    http::{StatusCode, HeaderMap},
};
use serde::Deserialize;

use crate::config::AppConfig;
use crate::storage::Storage;
use crate::xml_response;
use crate::auth;

#[derive(Clone)]
pub struct AppState {
    pub config: AppConfig,
    pub storage: Storage,
}

impl AppState {
    pub fn new(config: AppConfig) -> Self {
        let storage = Storage::new(&config);
        AppState { config, storage }
    }
}

pub async fn list_buckets(State(state): State<Arc<AppState>>, headers: HeaderMap) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    let buckets = state.storage.list_buckets();
    let xml = xml_response::list_buckets(&buckets);
    
    Ok(Response::builder()
        .header("Content-Type", "application/xml")
        .body(Body::from(xml))
        .unwrap())
}

pub async fn create_bucket(
    State(state): State<Arc<AppState>>,
    Path(bucket): Path<String>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    if bucket.contains("..") || bucket.contains('\\') {
        return Err(StatusCode::BAD_REQUEST);
    }

    state.storage.create_bucket(&bucket).map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(Response::builder().status(StatusCode::OK).body(Body::empty()).unwrap())
}

pub async fn delete_bucket(
    State(state): State<Arc<AppState>>,
    Path(bucket): Path<String>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    if !state.storage.bucket_exists(&bucket) {
        return Err(StatusCode::NOT_FOUND);
    }

    if !state.storage.bucket_empty(&bucket) {
        return Err(StatusCode::CONFLICT);
    }

    state.storage.delete_bucket(&bucket).map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(Response::builder().status(StatusCode::NO_CONTENT).body(Body::empty()).unwrap())
}

#[derive(Deserialize)]
pub struct ListObjectsQuery {
    prefix: Option<String>,
    #[serde(rename = "max-keys")]
    max_keys: Option<usize>,
}

pub async fn list_objects(
    State(state): State<Arc<AppState>>,
    Path(bucket): Path<String>,
    Query(query): Query<ListObjectsQuery>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    if !state.storage.bucket_exists(&bucket) {
        return Err(StatusCode::NOT_FOUND);
    }

    let prefix = query.prefix.as_deref().unwrap_or("");
    let max_keys = query.max_keys.unwrap_or(1000);

    let (objects, is_truncated) = state.storage.list_objects(&bucket, prefix, max_keys);
    let xml = xml_response::list_objects_v2(&objects, &bucket, prefix, max_keys, is_truncated);

    Ok(Response::builder()
        .header("Content-Type", "application/xml")
        .body(Body::from(xml))
        .unwrap())
}

pub async fn put_object(
    State(state): State<Arc<AppState>>,
    Path((bucket, key)): Path<(String, String)>,
    headers: HeaderMap,
    body: bytes::Bytes,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    if !state.storage.bucket_exists(&bucket) {
        return Err(StatusCode::NOT_FOUND);
    }

    let data = body.to_vec();
    let etag = state.storage.put_object(&bucket, &key, data)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(Response::builder()
        .header("ETag", format!("\"{}\"", etag))
        .status(StatusCode::OK)
        .body(Body::empty())
        .unwrap())
}

pub async fn get_object(
    State(state): State<Arc<AppState>>,
    Path((bucket, key)): Path<(String, String)>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    if !state.storage.object_exists(&bucket, &key) {
        return Err(StatusCode::NOT_FOUND);
    }

    let data = state.storage.get_object(&bucket, &key)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    Ok(Response::builder()
        .header("Content-Type", "application/octet-stream")
        .header("Accept-Ranges", "bytes")
        .body(Body::from(data))
        .unwrap())
}

pub async fn head_object(
    State(state): State<Arc<AppState>>,
    Path((bucket, key)): Path<(String, String)>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    let meta = state.storage.get_object_meta(&bucket, &key)
        .map_err(|_| StatusCode::NOT_FOUND)?;

    Ok(Response::builder()
        .header("Content-Length", meta.size.to_string())
        .header("Content-Type", meta.mime_type)
        .header("Last-Modified", format_http_date(meta.last_modified))
        .header("ETag", format!("\"{}\"", meta.etag))
        .header("Accept-Ranges", "bytes")
        .status(StatusCode::OK)
        .body(Body::empty())
        .unwrap())
}

pub async fn delete_object(
    State(state): State<Arc<AppState>>,
    Path((bucket, key)): Path<(String, String)>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    state.storage.delete_object(&bucket, &key).ok();

    Ok(Response::builder().status(StatusCode::NO_CONTENT).body(Body::empty()).unwrap())
}

pub async fn create_multipart_upload(
    State(state): State<Arc<AppState>>,
    Path((bucket, key)): Path<(String, String)>,
    headers: HeaderMap,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    let upload_id = generate_upload_id();
    state.storage.create_multipart_upload(&bucket, &upload_id)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    let xml = xml_response::create_multipart_upload(&bucket, &key, &upload_id);

    Ok(Response::builder()
        .header("Content-Type", "application/xml")
        .body(Body::from(xml))
        .unwrap())
}

pub async fn complete_multipart_upload(
    State(state): State<Arc<AppState>>,
    Path((bucket, key)): Path<(String, String)>,
    Query(params): Query<CompleteQuery>,
    headers: HeaderMap,
    body: bytes::Bytes,
) -> Result<Response<Body>, StatusCode> {
    auth::validate_signature(&headers, &state.config.access_keys)
        .map_err(|_| StatusCode::UNAUTHORIZED)?;

    let upload_id = params.uploadId.ok_or(StatusCode::BAD_REQUEST)?;

    if !state.storage.upload_exists(&bucket, &upload_id) {
        return Err(StatusCode::NOT_FOUND);
    }

    let xml_str = String::from_utf8_lossy(&body);
    let parts = parse_complete_xml(&xml_str);

    let mut part_datas = Vec::new();
    for part_num in 1..=parts.len() as u32 {
        let data = state.storage.get_part(&bucket, &upload_id, part_num)
            .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;
        part_datas.push(data);
    }

    let result = state.storage.complete_multipart_upload(&bucket, &key, &upload_id, &part_datas)
        .map_err(|_| StatusCode::INTERNAL_SERVER_ERROR)?;

    let xml = xml_response::complete_multipart_upload(&bucket, &key, &result.etag, result.size);

    Ok(Response::builder()
        .header("Content-Type", "application/xml")
        .body(Body::from(xml))
        .unwrap())
}

#[derive(Deserialize)]
pub struct CompleteQuery {
    #[serde(rename = "uploadId")]
    pub uploadId: Option<String>,
}

fn generate_upload_id() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let timestamp = SystemTime::now().duration_since(UNIX_EPOCH).unwrap();
    format!("{:x}{:x}", timestamp.as_secs(), timestamp.subsec_nanos())
}

fn parse_complete_xml(xml: &str) -> Vec<u32> {
    let mut parts = Vec::new();
    for line in xml.lines() {
        if line.contains("<PartNumber>") {
            if let Some(num) = line.split('>').nth(1) {
                if let Ok(n) = num.split('<').next().unwrap_or("").parse::<u32>() {
                    parts.push(n);
                }
            }
        }
    }
    parts
}

fn format_http_date(time_opt: Option<SystemTime>) -> String {
    use chrono::{DateTime, Utc};
    if let Some(time) = time_opt {
        let datetime: DateTime<Utc> = time.into();
        datetime.format("%a, %d %b %Y %H:%M:%S GMT").to_string()
    } else {
        String::new()
    }
}