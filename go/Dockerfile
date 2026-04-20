FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o s3server .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

RUN mkdir -p /app/data

COPY --from=builder /app/s3server .

EXPOSE 8000

ENV S3_DATA_DIR=/app/data

CMD ["./s3server"]