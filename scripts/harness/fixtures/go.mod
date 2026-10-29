// The fixtures' own module: with API_DIR=. (the Go module at the repo root, scripts/harness/layout.env) the
// root module's ./... then skips the fixtures' deliberate violations. No requires, no code of its own.
module fixtures
