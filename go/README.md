# Go S3 Server

S3-compatible object storage server in Go.

## Run Directly

```bash
go build -o s3server.exe
./s3server.exe
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
docker build -t go-s3 .
docker run -p 8000:8000 -v s3_data:/app/data -e S3_DATA_DIR=/app/data go-s3
```

## Operations

- Bucket: List, Create, Delete
- Object: Put, Get, Head, Delete
- Multipart: Create, UploadPart, Complete, Abort

## License

MIT