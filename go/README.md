# Go S3 Server

A lightweight S3-compatible object storage server written in Go.

## Features

- S3 API compatible (GET, PUT, HEAD, DELETE operations)
- Multipart uploads
- AWS Signature v4 authentication
- File-based storage
- Path traversal protection

## Quick Start

```bash
go build -o s3server.exe
./s3server.exe
```

Server runs on `http://0.0.0.0:8000`

## Configuration

| Environment Variable | Description | Default |
|-------------------|-------------|---------|
| `S3_PORT` | Server port | `8000` |
| `S3_HOST` | Server host | `0.0.0.0` |
| `S3_DATA_DIR` | Data directory | `./data` |
| `S3_ACCESS_KEY` | Access key ID | `minioadmin` |
| `S3_SECRET_KEY` | Secret key | `minioadmin` |
| `S3_DEBUG` | Enable debug logging | `false` |

## Usage with AWS CLI

```bash
# List buckets
aws s3 ls --endpoint http://localhost:8000 \
  --aws-access-key-id=minioadmin \
  --aws-secret-key=minioadmin \
  --region=us-east-1

# Create bucket
aws s3 mb s3://mybucket --endpoint http://localhost:8000 \
  --aws-access-key-id=minioadmin \
  --aws-secret-key=minioadmin \
  --region=us-east-1

# Upload file
aws s3 cp file.txt s3://mybucket/file.txt --endpoint http://localhost:8000 \
  --aws-access-key-id=minioadmin \
  --aws-secret-key=minioadmin \
  --region=us-east-1

# Download file
aws s3 cp s3://mybucket/file.txt file.txt --endpoint http://localhost:8000 \
  --aws-access-key-id=minioadmin \
  --aws-secret-key=minioadmin \
  --region=us-east-1
```

## API Endpoints

- `GET /` - List buckets
- `PUT /<bucket>` - Create bucket
- `DELETE /<bucket>` - Delete bucket
- `GET /<bucket>` - List objects in bucket
- `PUT /<bucket>/<key>` - Upload object
- `GET /<bucket>/<key>` - Download object
- `HEAD /<bucket>/<key>` - Get object metadata
- `DELETE /<bucket>/<key>` - Delete object

## Security

- AWS Signature v4 authentication required for all operations
- Path traversal protection
- Maximum file size: 10GB

## License

MIT