package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// CtxKeyAPIKeyPrincipal is the Fiber Locals key for M2M auth via X-API-Key (roadmap ш. 21).
const CtxKeyAPIKeyPrincipal = "merionyx.auth.api_key_principal"

// APIKeyRecordGetter loads API key metadata by SHA-256 hex digest of the secret (etcd path segment).
type APIKeyRecordGetter interface {
	Get(ctx context.Context, digestHex string) (kvvalue.APIKeyValue, int64, error)
}

// APIKeyPrincipal is a sanitized snapshot of the etcd record for RBAC (ш. 23). Raw secret is never stored.
type APIKeyPrincipal struct {
	DigestHex string
	Roles     []string
	Scopes    []string
}

// APIKeyPrincipalFromCtx returns the principal set by APISecurity after successful X-API-Key auth.
func APIKeyPrincipalFromCtx(c fiber.Ctx) (*APIKeyPrincipal, bool) {
	v, ok := c.Locals(CtxKeyAPIKeyPrincipal).(*APIKeyPrincipal)
	return v, ok && v != nil
}

func tryAPIKeyPrincipal(ctx context.Context, repo APIKeyRecordGetter, secret string) (*APIKeyPrincipal, error) {
	if repo == nil {
		return nil, nil
	}
	secret = trimAPIKeySecret(secret)
	if secret == "" {
		return nil, nil
	}
	digest := etcd.SHA256DigestHexFromSecret(secret)
	rec, _, err := repo.Get(ctx, digest)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return principalFromAPIKeyRecord(digest, rec), nil
}

func principalFromAPIKeyRecord(digestHex string, v kvvalue.APIKeyValue) *APIKeyPrincipal {
	return &APIKeyPrincipal{
		DigestHex: digestHex,
		Roles:     append([]string(nil), v.Roles...),
		Scopes:    append([]string(nil), v.Scopes...),
	}
}

func trimAPIKeySecret(s string) string {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, "\r\n") {
		return ""
	}
	return s
}

// Ensure *etcd.APIKeyRepository implements APIKeyRecordGetter.
var _ APIKeyRecordGetter = (*etcd.APIKeyRepository)(nil)
