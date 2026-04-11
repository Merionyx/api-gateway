// Package apiserver contains OpenAPI-derived types and Fiber v3 server glue
// (ServerInterface, RegisterHandlers, optional strict server types, GetSwagger) for the API Server HTTP API.
// With embedded-spec, GetSwagger supplies the same document used by OapiRequestValidator in internal/api-server/server.
//
// Regenerate after editing the spec (from repo root):
//
//	go generate -run oapi-codegen ./internal/api-server/gen/apiserver
//
// go generate runs each command with the working directory set to this package’s directory, so
// -o oapi_gen.go in the directive always writes next to this file.
//
// Requires oapi-codegen with fiber-v3-server and strict-server support (e.g. go install from your fork).
//
// Source: apis/rest/api-server/openapi.yaml
//
// The spec is pinned to OpenAPI 3.0.x for compatibility with oapi-codegen v2 (3.1 unions such as
// type: [string, null] are not supported).
package apiserver

//go:generate oapi-codegen -config oapi-codegen.yaml -o oapi_gen.go ../../../../apis/rest/api-server/openapi.yaml
