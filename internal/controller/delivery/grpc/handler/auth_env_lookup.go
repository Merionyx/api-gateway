package handler

import (
	"context"
	"time"

	"merionyx/api-gateway/internal/controller/index/bundleenv"
	ctrlmetrics "merionyx/api-gateway/internal/controller/metrics"
)

// AuthSchemaNotifyPlan is the outcome of resolving which environments to push after a schema (bundle) change.
type AuthSchemaNotifyPlan struct {
	TargetEnvironments []string
	NotifyAll          bool
}

// PlanAuthSchemaNotify resolves environments for a bundle key; NotifyAll is true when the index is missing or maps no envs after rebuild.
func PlanAuthSchemaNotify(bundleKey string, idx *bundleenv.Index, metricsEnabled bool) AuthSchemaNotifyPlan {
	envs := authEnvironmentsForBundleKey(bundleKey, idx, metricsEnabled)
	if len(envs) == 0 {
		return AuthSchemaNotifyPlan{NotifyAll: true}
	}
	return AuthSchemaNotifyPlan{TargetEnvironments: envs}
}

func authEnvironmentsForBundleKey(bundleKey string, idx *bundleenv.Index, metricsEnabled bool) []string {
	if idx == nil {
		return nil
	}
	envs := idx.EnvironmentsForBundle(bundleKey)
	if len(envs) > 0 {
		return envs
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	idx.Rebuild(ctx)
	cancel()
	ctrlmetrics.RecordBundleEnvIndexRebuild(metricsEnabled)
	return idx.EnvironmentsForBundle(bundleKey)
}
