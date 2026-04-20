# Rust S3 Server

S3-compatible object storage server in Rust.

## Run Directly

```bash
cargo build --release
./target/release/s3-server
```

Default port: 8000

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_PORT` | `8000` | Port |
| `S3_HOST` | `0.0.0.0` | Host |
| `S3_DATA_DIR` | `./data` | Data directory |
| `S3_ACCESS_KEY` | `minioadmin` | Access key |
| `S3_SECRET_KEY` | `minioadmin` | Secret key |
| `S3_DEBUG` | `false` | Debug mode |

## Run with Docker

```bash
# Build & run
docker-compose up -d

# Or build manually
docker build -t rust-s3 .
docker run -p 8000:8000 -v s3_data:/app/data -e S3_DATA_DIR=/app/data rust-s3
```

## Operations

- Bucket: List, Create, Delete
- Object: Put, Get, Head, Delete
- Multipart: Create, UploadPart, Complete, Abort

## License

MIT