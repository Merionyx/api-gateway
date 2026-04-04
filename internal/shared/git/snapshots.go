package git

import (
	"fmt"
	"log/slog"
	"sort"
)

func contractSnapshotsFromRepoFiles(files []RepositoryFile) ([]ContractSnapshot, error) {
	var snapshots []ContractSnapshot
	for _, file := range files {
		contractSchema, perr := parseContractSchema(file.Path, file.Content)
		if perr != nil {
			return nil, fmt.Errorf("parse schema %s: %w", file.Path, perr)
		}
		slog.Debug("parsed contract schema", "path", file.Path)

		if isXApiGatewayEmpty(contractSchema.XApiGateway) {
			slog.Debug("contract schema empty x-api-gateway block", "path", file.Path)
			continue
		}

		upstream := ContractUpstream{
			Name: contractSchema.XApiGateway.Service.Name,
		}

		snapshots = append(snapshots, ContractSnapshot{
			Name:                  contractSchema.XApiGateway.Contract.Name,
			Prefix:                contractSchema.XApiGateway.Prefix,
			Upstream:              upstream,
			AllowUndefinedMethods: contractSchema.XApiGateway.AllowUndefinedMethods,
			Access:                contractSchema.XApiGateway.Access,
		})
	}
	return snapshots, nil
}

func sortRepositoryFilesByPath(files []RepositoryFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}

func bundleSnapCacheKey(repoName, ref, path string) string {
	return repoName + "\x00" + ref + "\x00" + path
}

func cloneContractSnapshots(src []ContractSnapshot) []ContractSnapshot {
	out := make([]ContractSnapshot, len(src))
	for i := range src {
		out[i] = src[i]
		out[i].Access.Apps = append([]App(nil), src[i].Access.Apps...)
		for j := range out[i].Access.Apps {
			out[i].Access.Apps[j].Environments = append([]string(nil), src[i].Access.Apps[j].Environments...)
		}
	}
	return out
}

func normalizeContractSnapshotAccess(s ContractSnapshot) ContractSnapshot {
	apps := append([]App(nil), s.Access.Apps...)
	sort.Slice(apps, func(i, j int) bool { return apps[i].AppID < apps[j].AppID })
	for i := range apps {
		envs := append([]string(nil), apps[i].Environments...)
		sort.Strings(envs)
		apps[i].Environments = envs
	}
	s.Access.Apps = apps
	return s
}

func dedupeContractSnapshotsByName(snapshots []ContractSnapshot) []ContractSnapshot {
	byName := make(map[string]ContractSnapshot, len(snapshots))
	for _, s := range snapshots {
		s = normalizeContractSnapshotAccess(s)
		byName[s.Name] = s
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]ContractSnapshot, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}
