package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EffectiveMaterializedPrefix is the etcd key prefix for materialized effective
// (ADR 0001, phase 2). It is a sibling of EnvironmentPrefix, not a child of it.
// Full key: EffectiveMaterializedPrefix + "{name}/v1"
const EffectiveMaterializedPrefix = ControllerWatchPrefix + "effective/"

// MaterializedEffectiveV1 is the JSON document stored at …/effective/{name}/v1.
type MaterializedEffectiveV1 struct {
	SchemaVersion      int                                 `json:"schema_version"`
	Name               string                              `json:"name"`
	Type               string                              `json:"type"`
	Bundles            []models.StaticContractBundleConfig `json:"bundles"`
	Services           []models.StaticServiceConfig        `json:"services"`
	Generation         int64                               `json:"generation"`
	UpdatedAt          string                              `json:"updated_at"`
	SourcesFingerprint string                              `json:"sources_fingerprint"`
}

// MaterializedStore writes idempotent materialized effective documents.
type MaterializedStore struct {
	client *clientv3.Client
}

// NewMaterializedStore creates a store that writes under EffectiveMaterializedPrefix.
func NewMaterializedStore(client *clientv3.Client) *MaterializedStore {
	if client == nil {
		return nil
	}
	return &MaterializedStore{client: client}
}

// materializedV1Key returns the full etcd key for materialized v1 of an environment.
func materializedV1Key(environmentName string) string {
	if environmentName == "" || strings.Contains(environmentName, "/") {
		return ""
	}
	return EffectiveMaterializedPrefix + environmentName + "/v1"
}

// ReconcileIfChanged updates the key only when the fingerprint of the effective skeleton
// (static bundles and services) differs; bumps generation. No-op on unchanged fingerprint.
func (s *MaterializedStore) ReconcileIfChanged(ctx context.Context, skel *models.Environment) error {
	if s == nil || s.client == nil {
		return nil
	}
	if skel == nil {
		return fmt.Errorf("materialized: nil environment")
	}
	key := materializedV1Key(skel.Name)
	if key == "" {
		return fmt.Errorf("materialized: invalid environment name %q", skel.Name)
	}
	wantFP := envmodel.FingerprintStaticEnvironment(skel)

	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("get materialized: %w", err)
	}

	var next MaterializedEffectiveV1
	if len(resp.Kvs) == 0 {
		next.Generation = 1
	} else {
		if err := json.Unmarshal(resp.Kvs[0].Value, &next); err != nil {
			return fmt.Errorf("unmarshal materialized: %w", err)
		}
		if next.SourcesFingerprint == wantFP {
			return nil
		}
		next.Generation++
	}
	next.SchemaVersion = 1
	next.Name = skel.Name
	next.Type = skel.Type
	if skel.Bundles != nil {
		next.Bundles = make([]models.StaticContractBundleConfig, len(skel.Bundles.Static))
		copy(next.Bundles, skel.Bundles.Static)
	} else {
		next.Bundles = nil
	}
	if skel.Services != nil {
		next.Services = make([]models.StaticServiceConfig, len(skel.Services.Static))
		copy(next.Services, skel.Services.Static)
	} else {
		next.Services = nil
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	next.SourcesFingerprint = wantFP

	raw, err := json.Marshal(&next)
	if err != nil {
		return err
	}
	_, err = s.client.Put(ctx, key, string(raw))
	if err != nil {
		return fmt.Errorf("put materialized: %w", err)
	}
	return nil
}

// Delete removes the materialized v1 key. No-op if store/client is nil or name is invalid.
func (s *MaterializedStore) Delete(ctx context.Context, environmentName string) error {
	if s == nil || s.client == nil {
		return nil
	}
	key := materializedV1Key(environmentName)
	if key == "" {
		return nil
	}
	_, err := s.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("delete materialized: %w", err)
	}
	return nil
}

// Get returns the current materialized document, or nil if missing.
func (s *MaterializedStore) Get(ctx context.Context, environmentName string) (*MaterializedEffectiveV1, error) {
	if s == nil || s.client == nil {
		return nil, nil
	}
	key := materializedV1Key(environmentName)
	if key == "" {
		return nil, nil
	}
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get materialized: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var v MaterializedEffectiveV1
	if err := json.Unmarshal(resp.Kvs[0].Value, &v); err != nil {
		return nil, fmt.Errorf("unmarshal materialized: %w", err)
	}
	return &v, nil
}
