# Simple CDN

`simple_cdn` is a lightweight Content Delivery Network (CDN) server written in Go. 
It provides caching, request proxying, and diagnostic features. 
This application is designed to handle HTTP requests, cache responses based on configurable rules, 
and serve cached content to reduce the load on the upstream servers.

# Features
* Caching: Supports response caching with configurable rules for persistence and retrieval.
* Proxying: Forwards requests to an upstream server when caching is not applicable.
* Diagnostics: Includes a diagnostic server for health checks, metrics, and profiling.
* Logging and Metrics: Integrated logging and Prometheus metrics for observability.

# Installation
To build the simple_cdn server, you need to have Go installed. Clone the repository and run the following command:

```bash
go build -o simple_cdn main.go
```

But actually, you can use docker image `paragor/simple_cdn` that have both arm64 and amd64 architectures.


# Usage
Run the server with the following command:

```
LOG_LEVEL=info simple_cdn -config /path/to/config.yaml
```
## Command-Line Options
* `-config`: Path to the YAML configuration file (required).
* `-check-config`: Validates the configuration file and exits without starting the server.

# Configuration
See examples in [./examples/*.yaml](./examples)

## Configuration Parameters
- `listen_addr`: Address for the main server to listen on.
- `diagnostic_addr`: Address for the diagnostic server to listen on.
- `can_persist_cache`: Conditions under which responses can be cached.
- `can_load_cache`: Conditions under which cached responses can be served.
- `can_force_emit_debug_logging`: Conditions under which debug logging is forced.
- `cache`: Cache backend configuration (e.g., Redis).
- `cache_key_config`: Configuration for cache key generation based on cookies, headers, and query parameters.
- `upstream`: Configuration for the upstream server to which uncached requests are forwarded.

# Diagnostic Server
The diagnostic server provides the following endpoints:

- `/readyz`: Readiness probe endpoint.
- `/healthz`: Health check endpoint.
- `/invalidate`: Endpoint to invalidate cached content based on a pattern. (`/invalidate?pattern=/static/*` - not regexp)
- `/metrics`: Prometheus metrics endpoint.
- `/debug/pprof/`: pprof profiling endpoints for performance diagnostics.

# Logging
`simple_cdn` uses structured logging via `zap` for different log levels.
The log level can be configured through the `LOG_LEVEL` environment variable (`info`, `debug`, `error`, `warn`).

# Metrics
`simple_cdn` integrates with Prometheus for metrics collection.
Metrics are exposed at the /metrics endpoint of the diagnostic server.
