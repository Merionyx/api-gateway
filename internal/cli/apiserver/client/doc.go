// Package apiserverclient contains OpenAPI-derived request/response types and an HTTP client
// for the API Server REST API. Intended for agwctl; not wired into commands yet.
//
// Regenerate after editing the spec (from repo root):
//
//	go generate -run oapi-codegen ./internal/cli/apiclient/gen/apiserverclient
//
// go generate runs each command with the working directory set to this package’s directory, so
// -o client_gen.go in the directive always writes next to this file.
//
// Source: apis/rest/api-server/openapi.yaml
//
// The spec is pinned to OpenAPI 3.0.x for compatibility with oapi-codegen v2 (3.1 unions such as
// type: [string, null] are not supported).
package apiserverclient

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 -config oapi-codegen.yaml -o client_gen.go ../../../../apis/rest/api-server/openapi.yaml
