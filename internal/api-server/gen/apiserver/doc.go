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
// Regeneration uses the **`oapi-codegen` binary on PATH** (Merionyx fork with Fiber v3 templates: `fiber-v3-server`,
// `strict-server`, `embedded-spec`). Pin the tool to the fork revision you trust (e.g. commit
// `9a73536be6cdbc2ad87c3baacb2c9e55f1aad8e9` — `oapi-codegen -version` should report a build from that tree). If
// output is wrong or does not compile against Fiber v3, fix the fork; do not patch around generator bugs here.
//
// Source: apis/rest/api-server/openapi.yaml
//
// The spec is pinned to OpenAPI 3.0.x for compatibility with oapi-codegen v2 (3.1 unions such as
// type: [string, null] are not supported).
//
// The `swaggerSpec` blob in oapi_gen.go must be regenerated together with the rest of the file; if it
// drifts, GET handlers stay correct but GetSwagger / request validation may show an older document until
// you run codegen with embedded-spec enabled.
package apiserver

//go:generate oapi-codegen -config oapi-codegen.yaml -o oapi_gen.go ../../../../apis/rest/api-server/openapi.yaml
