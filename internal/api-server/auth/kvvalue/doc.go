// Package kvvalue defines JSON value shapes for auth etcd keys (sessions, login-intents, api-keys)
// under /api-gateway/api-server/auth/v1/… — see docs/etcd-auth-paths.md.
//
// Every value carries explicit schema_version. Parse* functions require known versions and reject
// missing/unsupported contracts. Marshal* always writes the explicit write-schema of each value type.
//
// This package does not perform etcd I/O (types + (de)serialization only).
package kvvalue
