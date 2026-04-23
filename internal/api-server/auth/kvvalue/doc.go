// Package kvvalue defines JSON value shapes for auth etcd keys (sessions, login-intents, api-keys)
// under /api-gateway/api-server/auth/v1/… — see docs/etcd-auth-paths.md.
//
// Every value carries schema_version (roadmap). Parse* functions apply on-read migration from v1 to v2;
// Marshal* always writes the latest schema (currently 2) for new writes.
//
// This package does not perform etcd I/O (roadmap step 9 — types + (de)serialization only).
package kvvalue
