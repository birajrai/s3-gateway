# S3 Compatible Server

A lightweight S3-compatible object storage server implementations in multiple languages.

## Implementations

| Language | Status | Path |
|----------|--------|------|
| PHP | Active | `php/` |
| Go | Active | `go/` |

## Quick Start

### PHP

```bash
cd php
# Configure .config.ini
# Upload to web server
```

### Go

```bash
cd go
go build -o s3server.exe
./s3server.exe
# Server runs on port 8000 by default
```

Environment variables:
- `S3_PORT` - Server port (default: 8000)
- `S3_HOST` - Server host (default: 0.0.0.0)
- `S3_DATA_DIR` - Data directory (default: ./data)
- `S3_SECRET_KEY` - Secret key for auth (default: minioadmin)
- `S3_DEBUG` - Enable debug logging (default: false)

See `php/README.md` for detailed documentation.

## License

MIT