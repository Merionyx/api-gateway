package git

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v6/memfs"
	"github.com/go-git/go-billy/v6/util"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"
)

func NewRepositoryManager() *RepositoryManager {
	return &RepositoryManager{
		repos:           make(map[string]*ManagedRepo),
		bundleSnapCache: make(map[string]bundleSnapshotCacheEntry),
	}
}

func (rm *RepositoryManager) InitializeRepositories(repos []RepositoryConfig) error {
	slog.Info("Initializing repositories", "repositories", repos)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, repository := range repos {
		slog.Info("Initializing repository", "name", repository.Name, "url", repository.URL, "auth", repository.Auth)

		switch repository.Source {
		case RepositorySourceGit:
			auth, err := resolveTransportAuth(repository.Auth, repository.Name)
			if err != nil {
				return err
			}

			repo, err := gogit.Clone(memory.NewStorage(), memfs.New(), &gogit.CloneOptions{
				URL:  repository.URL,
				Auth: auth,
			})
			if err != nil {
				return fmt.Errorf("failed to clone repository %s: %w", repository.Name, err)
			}

			rm.repos[repository.Name] = &ManagedRepo{
				Name:   repository.Name,
				Repo:   repo,
				Auth:   auth,
				Source: repository.Source,
				Path:   repository.Path,
			}

			slog.Info("Successfully cloned repository", "name", repository.Name)

		case RepositorySourceLocalGit:
			repo, err := gogit.Clone(memory.NewStorage(), memfs.New(), &gogit.CloneOptions{
				URL:          repository.Path,
				SingleBranch: false,
			})
			if err != nil {
				return fmt.Errorf("failed to clone local git repository %s: %w", repository.Name, err)
			}

			rm.repos[repository.Name] = &ManagedRepo{
				Name:   repository.Name,
				Repo:   repo,
				Auth:   nil,
				Source: repository.Source,
				Path:   repository.Path,
			}

			slog.Info("Successfully cloned local git repository to memory", "name", repository.Name)

		case RepositorySourceLocalDir:
			rm.repos[repository.Name] = &ManagedRepo{
				Name:   repository.Name,
				Repo:   nil,
				Auth:   nil,
				Source: repository.Source,
				Path:   repository.Path,
			}

			slog.Info("Successfully opened local directory", "name", repository.Name)

		default:
			return fmt.Errorf("unsupported repository source %q for repository %s", repository.Source, repository.Name)
		}
	}

	return nil
}

func (rm *RepositoryManager) GetRepository(name string) (*gogit.Repository, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	repo, ok := rm.repos[name]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", name)
	}
	return repo.Repo, nil
}

func (rm *RepositoryManager) getAuth(name string) (transport.AuthMethod, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	repo, ok := rm.repos[name]
	if !ok {
		return nil, fmt.Errorf("repository %s not found", name)
	}
	return repo.Auth, nil
}

func (rm *RepositoryManager) GetRepositorySnapshots(name string, ref string, path string) ([]ContractSnapshot, error) {
	rm.mu.RLock()
	managedRepo := rm.repos[name]
	rm.mu.RUnlock()

	if managedRepo == nil {
		return nil, fmt.Errorf("repository %s not found", name)
	}

	slog.Debug("getting repository snapshots", "repository", name, "ref", ref, "path", path)

	switch managedRepo.Source {
	case RepositorySourceLocalDir:
		return rm.getSnapshotsFromLocalDir(managedRepo.Path, path)
	case RepositorySourceGit, RepositorySourceLocalGit:
		return rm.getSnapshotsFromGit(managedRepo, ref, path)
	default:
		return nil, fmt.Errorf("unsupported repository source %s", managedRepo.Source)
	}
}

func isCommitHash(ref string) bool {
	if len(ref) != 40 {
		return false
	}
	_, ok := plumbing.FromHex(ref)
	return ok
}

func (rm *RepositoryManager) getSnapshotsFromGit(managedRepo *ManagedRepo, ref string, path string) ([]ContractSnapshot, error) {
	if path == "" {
		path = "."
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
		checkoutOptions = &gogit.CheckoutOptions{
			Hash: hash,
		}
	} else {
		checkoutOptions = &gogit.CheckoutOptions{
			Branch: plumbing.ReferenceName("refs/" + ref),
		}
	}

	if err = w.Checkout(checkoutOptions); err != nil {
		return nil, fmt.Errorf("failed to checkout repository %s: %w", managedRepo.Name, err)
	}

	auth, err := rm.getAuth(managedRepo.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth for repository %s: %w", managedRepo.Name, err)
	}

	switch managedRepo.Source {
	case RepositorySourceGit:
		if err = syncRemoteGitRepo(repo, w, auth, managedRepo.Name, ref); err != nil {
			return nil, err
		}
	default:
		if err = syncLocalGitWorktree(w, auth, managedRepo.Name); err != nil {
			return nil, err
		}
	}

	headRef, headErr := repo.Head()
	cacheKey := bundleSnapCacheKey(managedRepo.Name, ref, path)
	if headErr == nil {
		rm.cacheMu.RLock()
		ent, ok := rm.bundleSnapCache[cacheKey]
		rm.cacheMu.RUnlock()
		if ok && ent.commitHash == headRef.Hash().String() {
			slog.Debug("git bundle parse cache hit", "repository", managedRepo.Name, "ref", ref, "path", path, "commit", ent.commitHash)
			return cloneContractSnapshots(ent.snapshots), nil
		}
	}

	var files []RepositoryFile

	err = util.Walk(w.Filesystem, path, func(filePath string, info os.FileInfo, walkErr error) error {
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

		files = append(files, RepositoryFile{
			Path:    filePath,
			Content: content,
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for repository %s: %w", managedRepo.Name, err)
	}

	sortRepositoryFilesByPath(files)

	snapshots, err := contractSnapshotsFromRepoFiles(files)
	if err != nil {
		return nil, err
	}

	snapshots = dedupeContractSnapshotsByName(snapshots)

	if headErr == nil {
		rm.cacheMu.Lock()
		rm.bundleSnapCache[cacheKey] = bundleSnapshotCacheEntry{
			commitHash: headRef.Hash().String(),
			snapshots:  cloneContractSnapshots(snapshots),
		}
		rm.cacheMu.Unlock()
	}

	return snapshots, nil
}

func syncRemoteGitRepo(repo *gogit.Repository, w *gogit.Worktree, auth transport.AuthMethod, name, ref string) error {
	fetchErr := repo.Fetch(&gogit.FetchOptions{
		RemoteName: gogit.DefaultRemoteName,
		Auth:       auth,
	})
	if fetchErr != nil && fetchErr != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch repository %s: %w", name, fetchErr)
	}
	if fetchErr == gogit.NoErrAlreadyUpToDate {
		slog.Debug("git remote unchanged, skip pull", "repository", name, "ref", ref)
		return nil
	}
	if err := w.Pull(&gogit.PullOptions{Force: true, Auth: auth}); err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull repository %s: %w", name, err)
	}
	slog.Debug("pulled repository", "repository", name)
	return nil
}

func syncLocalGitWorktree(w *gogit.Worktree, auth transport.AuthMethod, name string) error {
	err := w.Pull(&gogit.PullOptions{
		Force: true,
		Auth:  auth,
	})
	if err != nil {
		if err != gogit.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to pull repository %s: %w", name, err)
		}
		slog.Debug("repository already up to date", "repository", name)
	}
	slog.Debug("pulled repository", "repository", name)
	return nil
}

func (rm *RepositoryManager) getSnapshotsFromLocalDir(basePath, subPath string) ([]ContractSnapshot, error) {
	fullPath := filepath.Join(basePath, subPath)
	var files []RepositoryFile

	err := filepath.Walk(fullPath, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !info.IsDir() && isSchemaFile(p) {
			content, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			files = append(files, RepositoryFile{
				Path:    p,
				Content: content,
			})
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory for repository %s: %w", basePath, err)
	}

	sortRepositoryFilesByPath(files)

	snapshots, err := contractSnapshotsFromRepoFiles(files)
	if err != nil {
		return nil, err
	}

	return dedupeContractSnapshotsByName(snapshots), nil
}
