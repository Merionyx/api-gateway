package usecase

import (
	"context"
	"log/slog"

	"github.com/merionyx/api-gateway/internal/controller/config"
	ctrlmetrics "github.com/merionyx/api-gateway/internal/controller/metrics"
)

// Constants kind — metrics (labels) and comparisons in tests; stable strings, do not leak PII.
const (
	// RegistryBuildWarningInMemoryList — in-memory [interfaces.InMemoryEnvironmentsRepository.ListEnvironments] failed.
	RegistryBuildWarningInMemoryList = "in_memory_env_list"
	// RegistryBuildWarningEtcdList — controller etcd [interfaces.EnvironmentRepository.ListEnvironments] failed.
	RegistryBuildWarningEtcdList = "etcd_env_list"
	// RegistryBuildWarningEnvMerge — merge effective env for name is not possible (skip, partial payload).
	RegistryBuildWarningEnvMerge = "environment_merge_skip"
	// RegistryBuildWarningMaterializedGet — read materialized generation in EnvironmentMeta.
	RegistryBuildWarningMaterializedGet = "materialized_get"
	// RegistryBuildWarningListContractSnapshots — [SchemaRepository.ListContractSnapshots] in snapshot merge helper.
	RegistryBuildWarningListContractSnapshots = "list_contract_snapshots"
)

// RegistryEnvironmentsBuildWarning — one degradation during build: metric by kind; Subject — env or subsystem.
type RegistryEnvironmentsBuildWarning struct {
	Kind    string
	Subject string
	Err     error
}

// RegistryEnvironmentsBuildReport accompanies [registryEnvironmentsBuilder.buildEnvironmentsForAPIServer] and
// list of names in follower: Warnings != nil — partial/weakened slice, not to be confused with «normally empty».
type RegistryEnvironmentsBuildReport struct {
	Warnings []RegistryEnvironmentsBuildWarning
}

func (r *RegistryEnvironmentsBuildReport) addWarning(kind, subject string, err error) {
	if kind == "" || err == nil {
		return
	}
	r.Warnings = append(r.Warnings, RegistryEnvironmentsBuildWarning{Kind: kind, Subject: subject, Err: err})
}

func (r *RegistryEnvironmentsBuildReport) appendNameListWarnings(w []RegistryEnvironmentsBuildWarning) {
	r.Warnings = append(r.Warnings, w...)
}

// degraded reports whether any non-fatal issue occurred (partial name list, skipped env, …).
func (r *RegistryEnvironmentsBuildReport) degraded() bool {
	return len(r.Warnings) > 0
}

func countWarningKinds(w []RegistryEnvironmentsBuildWarning) map[string]int {
	m := make(map[string]int)
	for _, e := range w {
		if e.Kind != "" {
			m[e.Kind]++
		}
	}
	return m
}

// registryOp* — op for aggregated log: heartbeat does not spam Warn, only metric + debug.
const (
	registryOpRegister    = "register"
	registryOpHeartbeat   = "heartbeat"
	registryOpFollowerXDS = "follower_rebuild"
)

// observeRegistryEnvironmentsBuildDegradation: **metric** per kind; **log** — one aggregated string
// (do not duplicate per-warning in slog on the same path as counters). P.5 backlog.
func observeRegistryEnvironmentsBuildDegradation(ctx context.Context, cfg *config.Config, op string, report *RegistryEnvironmentsBuildReport) {
	if cfg == nil || report == nil || !report.degraded() {
		return
	}
	en := cfg.MetricsHTTP.Enabled
	for _, w := range report.Warnings {
		ctrlmetrics.RecordRegistryEnvironmentsBuildWarning(en, w.Kind)
	}
	kinds := countWarningKinds(report.Warnings)
	switch op {
	case registryOpHeartbeat:
		if ctx == nil {
			ctx = context.Background()
		}
		slog.Log(ctx, slog.LevelDebug, "registry build degraded (heartbeat, metrics per kind set)",
			"op", op, "kinds", kinds, "warning_count", len(report.Warnings))
	case registryOpFollowerXDS:
		slog.Warn("env name set or registry merge may be incomplete; see metrics",
			"op", op, "kinds", kinds, "warning_count", len(report.Warnings),
			"first_err", firstWarningErrString(report))
	default:
		slog.Warn("registry build degraded; see metrics",
			"op", op, "kinds", kinds, "warning_count", len(report.Warnings),
			"first_err", firstWarningErrString(report))
	}
}

func firstWarningErrString(r *RegistryEnvironmentsBuildReport) any {
	if r == nil || len(r.Warnings) == 0 {
		return nil
	}
	if r.Warnings[0].Err != nil {
		return r.Warnings[0].Err.Error()
	}
	return nil
}

// follower's collectEnvironmentNames: one log + metrics per path, if lists are partial.
func observeNameListDegradationForFollower(_ context.Context, cfg *config.Config, nameListSource []RegistryEnvironmentsBuildWarning) {
	if len(nameListSource) == 0 || cfg == nil {
		return
	}
	en := cfg.MetricsHTTP.Enabled
	for _, w := range nameListSource {
		ctrlmetrics.RecordRegistryEnvironmentsBuildWarning(en, w.Kind)
	}
	kinds := countWarningKinds(nameListSource)
	slog.Warn("environment name set for xDS may be incomplete (list partial failure); see registry_environments_build_warnings",
		"op", registryOpFollowerXDS, "kinds", kinds, "n", len(nameListSource), "first_err", firstInSliceErr(nameListSource))
}

func firstInSliceErr(w []RegistryEnvironmentsBuildWarning) any {
	for _, e := range w {
		if e.Err != nil {
			return e.Err.Error()
		}
	}
	return nil
}
