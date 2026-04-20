use hmac::{Hmac, Mac};
use http::HeaderMap;
use sha2::{Digest, Sha256};
use std::collections::HashMap;

type HmacSha256 = Hmac<Sha256>;

const MAX_TIMESTAMP_SKEW: i64 = 300;

pub fn validate_signature(
    headers: &HeaderMap,
    access_keys: &HashMap<String, String>,
) -> Result<String, String> {
    let auth_header = headers
        .get("authorization")
        .and_then(|v| v.to_str().ok())
        .ok_or("Missing Authorization header")?;

    if !auth_header.starts_with("AWS4-HMAC-SHA256") {
        return Err("Invalid Authorization header".to_string());
    }

    let parts = parse_auth_header(auth_header)?;
    let access_key_id = parts
        .get("AccessKeyId")
        .cloned()
        .ok_or("Missing AccessKeyId")?;

    let secret_key = access_keys
        .get(&access_key_id)
        .ok_or("Invalid AccessKeyId")?;

    validate_timestamp(headers)?;

    let method = headers
        .get("x-amz-content-sha256")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("UNSIGNED-PAYLOAD");

    let string_to_sign = build_string_to_sign(headers, method, &parts);
    let calculated_signature = calculate_signature(&string_to_sign, secret_key, &parts);

    if calculated_signature == parts["Signature"] {
        Ok(access_key_id.clone())
    } else {
        Err("SignatureDoesNotMatch".to_string())
    }
}

fn parse_auth_header(auth: &str) -> Result<HashMap<String, String>, String> {
    let mut parts = HashMap::new();

    for part in auth.split(',') {
        let part = part.trim();
        if let Some((key, value)) = part.split_once('=') {
            parts.insert(key.trim().to_string(), value.trim().to_string());
        }
    }

    if let Some(cred) = parts.get("Credential") {
        let cred_str = cred.as_str();
        let cred_parts: Vec<&str> = cred_str.split('/').collect();
        if cred_parts.len() >= 5 {
            let ak = cred_parts[0].to_string();
            let date = cred_parts[1].to_string();
            let region = cred_parts[2].to_string();
            let service = cred_parts[3].to_string();
            parts.insert("AccessKeyId".to_string(), ak);
            parts.insert("Date".to_string(), date);
            parts.insert("Region".to_string(), region);
            parts.insert("Service".to_string(), service);
        }
    }

    Ok(parts)
}

fn validate_timestamp(headers: &HeaderMap) -> Result<(), String> {
    let amz_date = headers
        .get("x-amz-date")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.replace("Z", ""))
        .or_else(|| {
            headers
                .get("date")
                .and_then(|v| v.to_str().ok())
                .map(|s| {
                    chrono::DateTime::parse_from_rfc2822(s)
                        .map(|dt| dt.format("%Y%m%dT%H%M%S").to_string())
                        .ok()
                })
                .flatten()
        });

    let amz_date = amz_date.ok_or("Missing X-Amz-Date header")?;

    let request_time = chrono::NaiveDateTime::parse_from_str(&amz_date, "%Y%m%dT%H%M%S")
        .map_err(|_| "Invalid X-Amz-Date format")?;

    let now = chrono::Utc::now().naive_utc();
    let diff = (now - request_time).num_seconds().abs();

    if diff > MAX_TIMESTAMP_SKEW {
        return Err("Request timestamp skew too large".to_string());
    }

    Ok(())
}

fn build_string_to_sign(
    headers: &HeaderMap,
    payload_hash: &str,
    parts: &HashMap<String, String>,
) -> String {
    let method = headers
        .get(":method")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("GET");

    let uri = headers
        .get(":path")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("/");

    let query = headers
        .get(":path")
        .and_then(|v| v.to_str().ok())
        .and_then(|s| s.split('?').nth(1))
        .unwrap_or("");

    let signed_headers = parts.get("SignedHeaders").cloned().unwrap_or_default();

    let canonical_headers = build_canonical_headers(headers, &signed_headers);

    let canonical_request = format!(
        "{}\n{}\n{}\n{}\n\n{}\n{}",
        method, uri, query, canonical_headers, signed_headers, payload_hash
    );

    let amz_date = parts
        .get("Date")
        .cloned()
        .unwrap_or_else(|| chrono::Utc::now().format("%Y%m%dT%H%M%S").to_string());

    let date = &amz_date[..8];
    let region = parts
        .get("Region")
        .cloned()
        .unwrap_or_else(|| "us-east-1".to_string());
    let service = parts
        .get("Service")
        .cloned()
        .unwrap_or_else(|| "s3".to_string());

    let hashed_request = hash_sha256(canonical_request.as_bytes());

    format!(
        "AWS4-HMAC-SHA256\n{}\n{}/{}/aws4_request\n{}",
        amz_date, date, region, service
    ) + "\n"
        + &hashed_request
}

fn build_canonical_headers(headers: &HeaderMap, signed_headers: &str) -> String {
    let mut header_list: Vec<&str> = signed_headers.split(';').collect();
    header_list.sort();

    let mut canonical = Vec::new();
    for header in header_list {
        let header = header.trim();
        if let Some(value) = headers.get(header).and_then(|v| v.to_str().ok()) {
            canonical.push(format!("{}:{}", header, value.trim()));
        }
    }

    canonical.join("\n")
}

fn calculate_signature(
    string_to_sign: &str,
    secret_key: &str,
    parts: &HashMap<String, String>,
) -> String {
    let amz_date = parts
        .get("Date")
        .cloned()
        .unwrap_or_else(|| chrono::Utc::now().format("%Y%m%dT%H%M%S").to_string());

    let date = &amz_date[..8];
    let region = parts
        .get("Region")
        .cloned()
        .unwrap_or_else(|| "us-east-1".to_string());
    let service = parts
        .get("Service")
        .cloned()
        .unwrap_or_else(|| "s3".to_string());

    let mut mac = HmacSha256::new_from_slice(format!("AWS4{}", secret_key).as_bytes()).unwrap();
    mac.update(date.as_bytes());
    let k_date = mac.finalize().into_bytes();

    let mut mac = HmacSha256::new_from_slice(&k_date).unwrap();
    mac.update(region.as_bytes());
    let k_region = mac.finalize().into_bytes();

    let mut mac = HmacSha256::new_from_slice(&k_region).unwrap();
    mac.update(service.as_bytes());
    let k_service = mac.finalize().into_bytes();

    let mut mac = HmacSha256::new_from_slice(&k_service).unwrap();
    mac.update(b"aws4_request");
    let k_signing = mac.finalize().into_bytes();

    let mut mac = HmacSha256::new_from_slice(&k_signing).unwrap();
    mac.update(string_to_sign.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}

fn hash_sha256(data: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(data);
    hex::encode(hasher.finalize())
}
