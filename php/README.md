# PHP S3 Server

S3-compatible object storage server in PHP.

## Run Directly

Upload to a web server, configure `.config.ini`:

```ini
[general]
DATA_DIR=./data

[keys.key1]
secret_key=your-secret-key
allowed_buckets=bucket1,bucket2
file_max_size=10240
```

## Run with Docker

```bash
# Build & run
docker-compose up -d

# Or build manually
docker build -t php-s3 .
docker run -p 8000:80 -v s3_data:/var/www/html/data php-s3
```

## Environment Variables (Docker)

| Variable | Default | Description |
|----------|---------|-------------|
| `DATA_DIR` | `/var/www/html/data` | Data directory |
| `APP_DEBUG` | `false` | Debug mode |
| `S3_ACCESS_KEY` | `minioadmin` | Access key |
| `S3_SECRET_KEY` | `minioadmin` | Secret key |

## Operations

- Bucket: List, Create, Delete
- Object: Put, Get, Head, Delete, Copy
- Multipart: Create, UploadPart, Complete, Abort

## License

MIT