use std::collections::HashMap;
use std::env;
use std::net::IpAddr;
use std::path::PathBuf;

#[derive(Clone)]
pub struct AppConfig {
    pub port: u16,
    pub host: IpAddr,
    pub data_dir: PathBuf,
    pub access_keys: HashMap<String, String>,
    pub debug: bool,
}

pub fn load_config() -> AppConfig {
    let port = env::var("S3_PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(8000);

    let host = env::var("S3_HOST")
        .ok()
        .and_then(|h| h.parse().ok())
        .unwrap_or("0.0.0.0".parse().unwrap());

    let data_dir = env::var("S3_DATA_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|_| PathBuf::from("./data"));

    let access_key = env::var("S3_ACCESS_KEY").unwrap_or_else(|_| "minioadmin".to_string());

    let secret_key = env::var("S3_SECRET_KEY").unwrap_or_else(|_| "minioadmin".to_string());

    let mut access_keys = HashMap::new();
    access_keys.insert(access_key, secret_key);

    let debug = env::var("S3_DEBUG")
        .map(|v| v == "true" || v == "1")
        .unwrap_or(false);

    std::fs::create_dir_all(&data_dir).ok();

    AppConfig {
        port,
        host,
        data_dir,
        access_keys,
        debug,
    }
}
