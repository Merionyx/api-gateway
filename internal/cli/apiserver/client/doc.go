// Package apiserverclient contains OpenAPI-derived request/response types and an HTTP client
// for the API Server REST API (used by internal/cli/apiserver helpers).
//
// Regenerate after editing the spec (from repo root):
//
//	go generate -run oapi-codegen ./internal/cli/apiserver/client
//
// go generate runs each command with the working directory set to this package’s directory, so
// -o client_gen.go in the directive always writes next to this file.
//
// Source: apis/rest/api-server/openapi.yaml
//
// The spec is pinned to OpenAPI 3.0.x for compatibility with oapi-codegen v2 (3.1 unions such as
// type: [string, null] are not supported).
//
// Regeneration uses the **`oapi-codegen` binary on PATH** (Merionyx fork with Fiber v3; pin e.g. commit
// `9a73536be6cdbc2ad87c3baacb2c9e55f1aad8e9`). Bad or incompatible generated code is addressed in that fork, not
// worked around in this repo.
package apiserverclient

//go:generate oapi-codegen -config oapi-codegen.yaml -o client_gen.go ../../../../apis/rest/api-server/openapi.yaml
