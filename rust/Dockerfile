FROM rust:1.75-alpine AS builder

WORKDIR /app

COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs

RUN cargo build --release
RUN rm -rf target/release/deps/*.d

COPY . .
RUN cargo build --release

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/target/release/s3-server .

EXPOSE 8000

CMD ["./s3-server"]