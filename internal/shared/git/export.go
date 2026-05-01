package git

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v6/util"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// ExportedContractFile is raw schema file content keyed by logical contract name from x-api-gateway.
type ExportedContractFile struct {
	ContractName string
	SourcePath   string
	Content      []byte
}

// ExportContractFiles returns raw OpenAPI files for contracts under path (empty = repository root).
// contractNameFilter empty means all contracts; otherwise only that name.
// Duplicate contract names across different files under the same walk root return an error.
func (rm *RepositoryManager) ExportContractFiles(ctx context.Context, name string, ref string, path string, contractNameFilter string) (out []ExportedContractFile, err error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanGitPkg, "ExportContractFiles"))
	defer func() {
		telemetry.MarkError(span, err)
		span.End()
	}()
	rm.mu.RLock()
	managedRepo := rm.repos[name]
	rm.mu.RUnlock()

	if managedRepo == nil {
		return nil, fmt.Errorf("repository %s not found", name)
	}

	switch managedRepo.Source {
	case RepositorySourceLocalDir:
		return rm.exportContractFilesFromLocalDir(managedRepo.Path, path, contractNameFilter)
	case RepositorySourceGit, RepositorySourceLocalGit:
		return rm.exportContractFilesFromGit(managedRepo, ref, path, contractNameFilter)
	default:
		return nil, fmt.Errorf("unsupported repository source %s", managedRepo.Source)
	}
}

func (rm *RepositoryManager) exportContractFilesFromLocalDir(basePath, subPath, contractNameFilter string) ([]ExportedContractFile, error) {
	walkRoot := basePath
	if strings.TrimSpace(subPath) != "" {
		walkRoot = filepath.Join(basePath, subPath)
	}
	root, err := os.OpenRoot(walkRoot)
	if err != nil {
		return nil, fmt.Errorf("open root %s: %w", walkRoot, err)
	}
	defer func() { _ = root.Close() }()

	var files []RepositoryFile
	err = filepath.WalkDir(walkRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !isSchemaFile(p) {
			return nil
		}
		relToWalkRoot, rerr := filepath.Rel(walkRoot, p)
		if rerr != nil {
			return fmt.Errorf("rel path %s: %w", p, rerr)
		}
		content, rerr := root.ReadFile(relToWalkRoot)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(basePath, p)
		if rel == "." {
			rel = filepath.Base(p)
		}
		files = append(files, RepositoryFile{Path: rel, Content: content})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for repository %s: %w", basePath, err)
	}

	sortRepositoryFilesByPath(files)
	return finalizeExportFiles(files, contractNameFilter)
}

func (rm *RepositoryManager) exportContractFilesFromGit(managedRepo *ManagedRepo, ref string, path string, contractNameFilter string) ([]ExportedContractFile, error) {
	walkRoot := path
	if walkRoot == "" {
		walkRoot = "."
	}

	repo := managedRepo.Repo
	w, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree for repository %s: %w", managedRepo.Path, err)
	}

	var checkoutOptions *gogit.CheckoutOptions
	if isCommitHash(ref) {
		hash, ok := plumbing.FromHex(ref)
		if !ok {
			return nil, fmt.Errorf("invalid commit hash %q", ref)
		}
		checkoutOptions = &gogit.CheckoutOptions{Hash: hash}
	} else {
		checkoutOptions = &gogit.CheckoutOptions{
			Branch: resolveCheckoutRefName(ref),
		}
	}

	if err = w.Checkout(checkoutOptions); err != nil {
		return nil, fmt.Errorf("failed to checkout repository %s: %w", managedRepo.Name, err)
	}

	clientOpts, err := rm.getClientOptions(managedRepo.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth for repository %s: %w", managedRepo.Name, err)
	}

	switch managedRepo.Source {
	case RepositorySourceGit:
		if err = syncRemoteGitRepo(repo, w, clientOpts, managedRepo.Name, ref); err != nil {
			return nil, err
		}
	default:
		if err = syncLocalGitWorktree(w, clientOpts, managedRepo.Name); err != nil {
			return nil, err
		}
	}

	var files []RepositoryFile
	err = util.Walk(w.Filesystem, walkRoot, func(filePath string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !isSchemaFile(filePath) {
			return nil
		}
		f, oerr := w.Filesystem.Open(filePath)
		if oerr != nil {
			return fmt.Errorf("failed to open file %s: %w", filePath, oerr)
		}
		content, rerr := io.ReadAll(f)
		_ = f.Close()
		if rerr != nil {
			return fmt.Errorf("failed to read file %s: %w", filePath, rerr)
		}
		files = append(files, RepositoryFile{Path: filePath, Content: content})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for repository %s: %w", managedRepo.Name, err)
	}

	sortRepositoryFilesByPath(files)
	return finalizeExportFiles(files, contractNameFilter)
}

type exportEntry struct {
	path    string
	content []byte
}

func finalizeExportFiles(files []RepositoryFile, contractNameFilter string) ([]ExportedContractFile, error) {
	byName := make(map[string][]exportEntry)

	for _, file := range files {
		contractSchema, perr := parseContractSchema(file.Path, file.Content)
		if perr != nil {
			return nil, fmt.Errorf("parse schema %s: %w", file.Path, perr)
		}
		if isXApiGatewayEmpty(contractSchema.XApiGateway) {
			slog.Debug("export: skip empty x-api-gateway", "path", file.Path)
			continue
		}
		cn := strings.TrimSpace(contractSchema.XApiGateway.Contract.Name)
		if cn == "" {
			return nil, fmt.Errorf("contract name missing in x-api-gateway (file %s)", file.Path)
		}
		byName[cn] = append(byName[cn], exportEntry{path: file.Path, content: append([]byte(nil), file.Content...)})
	}

	for name, entries := range byName {
		if len(entries) > 1 {
			paths := make([]string, len(entries))
			for i := range entries {
				paths[i] = entries[i].path
			}
			sort.Strings(paths)
			return nil, fmt.Errorf("duplicate contract name %q in files: %s", name, strings.Join(paths, ", "))
		}
	}

	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	var out []ExportedContractFile
	for _, n := range names {
		if contractNameFilter != "" && n != contractNameFilter {
			continue
		}
		e := byName[n][0]
		out = append(out, ExportedContractFile{
			ContractName: n,
			SourcePath:   e.path,
			Content:      e.content,
		})
	}

	if contractNameFilter != "" && len(out) == 0 {
		return nil, fmt.Errorf("contract %q not found", contractNameFilter)
	}

	return out, nil
}
