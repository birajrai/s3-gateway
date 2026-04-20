mod config;
mod storage;
mod handlers;
mod auth;
mod xml_response;

use std::net::SocketAddr;
use std::sync::Arc;
use axum::{Router, routing::{get, put, delete, head, post}};
use tower_http::trace::TraceLayer;
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

#[tokio::main]
async fn main() {
    tracing_subscriber::registry()
        .with(tracing_subscriber::fmt::layer())
        .init();

    let config = config::load_config();
    let state = Arc::new(handlers::AppState::new(config));

    let app = Router::new()
        .route("/", get(handlers::list_buckets))
        .route("/:bucket", get(handlers::list_objects).put(handlers::create_bucket).delete(handlers::delete_bucket))
        .route("/:bucket/", get(handlers::list_objects))
        .route("/:bucket/:key", get(handlers::get_object).put(handlers::put_object).delete(handlers::delete_object).head(handlers::head_object))
        .route("/:bucket/:key/uploads", post(handlers::create_multipart_upload))
        .route("/:bucket/:key", post(handlers::complete_multipart_upload))
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], 8000));
    tracing::info!("Server starting on {}", addr);

    let listener = tokio::net::TcpListener::bind(addr).await.unwrap();
    axum::serve(listener, app).await.unwrap();
}