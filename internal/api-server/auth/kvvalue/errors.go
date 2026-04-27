package kvvalue

import "errors"

var (
	// ErrMissingSchemaVersion is returned when schema_version is absent or not a positive int.
	ErrMissingSchemaVersion = errors.New("kvvalue: schema_version missing or invalid")

	// ErrUnsupportedSessionSchema is returned for unknown session schema_version.
	ErrUnsupportedSessionSchema = errors.New("kvvalue: unsupported session schema_version")

	// ErrUnsupportedLoginIntentSchema is returned for unknown login-intent schema_version.
	ErrUnsupportedLoginIntentSchema = errors.New("kvvalue: unsupported login-intent schema_version")

	// ErrUnsupportedAPIKeySchema is returned for unknown api-key schema_version.
	ErrUnsupportedAPIKeySchema = errors.New("kvvalue: unsupported api-key schema_version")

	// ErrUnsupportedTokenGrantSchema is returned for unknown token-grant schema_version.
	ErrUnsupportedTokenGrantSchema = errors.New("kvvalue: unsupported token-grant schema_version")
)
