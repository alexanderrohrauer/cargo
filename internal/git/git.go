package git

import (
	"fmt"
	"log/slog"
	"os"

	"cargo/internal/config"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// authMethod returns the appropriate go-git transport auth for the project config.
func authMethod(cfg config.GitConfig) (transport.AuthMethod, error) {
	if cfg.SSHKey != "" {
		auth, err := ssh.NewPublicKeysFromFile("git", cfg.SSHKey, "")
		if err != nil {
			return nil, fmt.Errorf("loading SSH key from %q: %w", cfg.SSHKey, err)
		}
		return auth, nil
	}
	if cfg.Username != "" || cfg.Password != "" {
		return &http.BasicAuth{
			Username: cfg.Username,
			Password: cfg.Password,
		}, nil
	}
	return nil, nil
}

// Clone clones the repository specified in projectCfg into destDir.
func Clone(projectCfg config.ProjectConfig, destDir string) error {
	auth, err := authMethod(projectCfg.Git)
	if err != nil {
		return fmt.Errorf("resolving auth for project %q: %w", projectCfg.Name, err)
	}

	cloneOpts := &gogit.CloneOptions{
		URL:      projectCfg.Git.URL,
		Auth:     auth,
		Progress: os.Stdout,
	}

	slog.Info("cloning repository", "url", projectCfg.Git.URL, "dest", destDir)
	repo, err := gogit.PlainClone(destDir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("cloning %q into %q: %w", projectCfg.Git.URL, destDir, err)
	}

	if projectCfg.Git.Revision != "" {
		if err := Checkout(repo, projectCfg.Git.Revision); err != nil {
			return fmt.Errorf("checking out revision %q: %w", projectCfg.Git.Revision, err)
		}
	}

	return nil
}

// Pull fetches the latest changes from the remote and hard-resets the worktree
// to the remote ref. Using fetch+reset instead of pull avoids go-git's
// "worktree contains unstaged changes" check, which would fail when generated
// files (e.g. SOPS-decrypted secrets) are present inside the repo directory.
func Pull(projectCfg config.ProjectConfig, repoDir string) error {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("opening repo at %q: %w", repoDir, err)
	}

	auth, err := authMethod(projectCfg.Git)
	if err != nil {
		return fmt.Errorf("resolving auth for project %q: %w", projectCfg.Name, err)
	}

	slog.Info("fetching repository", "dir", repoDir)
	fetchErr := repo.Fetch(&gogit.FetchOptions{
		Auth:     auth,
		Progress: os.Stdout,
		Force:    true,
	})
	if fetchErr != nil && fetchErr != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("fetching %q: %w", repoDir, fetchErr)
	}

	// Resolve the remote tracking ref to get the commit hash to reset to.
	remoteRef, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
	if err != nil {
		// Fall back to the configured revision or plain HEAD.
		remoteRef, err = repo.Head()
		if err != nil {
			return fmt.Errorf("resolving HEAD for %q: %w", repoDir, err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	// Hard-reset to the remote commit — untracked files are left in place but
	// tracked files are restored, and the unstaged-changes check is skipped.
	if err := wt.Reset(&gogit.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   gogit.HardReset,
	}); err != nil {
		return fmt.Errorf("resetting %q to %s: %w", repoDir, remoteRef.Hash(), err)
	}

	if projectCfg.Git.Revision != "" {
		if err := Checkout(repo, projectCfg.Git.Revision); err != nil {
			return fmt.Errorf("checking out revision %q after fetch: %w", projectCfg.Git.Revision, err)
		}
	}

	return nil
}

// Checkout checks out the given revision (branch, tag, or commit hash) in the repo.
func Checkout(repo *gogit.Repository, revision string) error {
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	// Try branch first
	branchRef := plumbing.NewBranchReferenceName(revision)
	checkoutOpts := &gogit.CheckoutOptions{
		Branch: branchRef,
		Force:  true,
	}

	err = wt.Checkout(checkoutOpts)
	if err == nil {
		return nil
	}

	// Try tag
	tagRef := plumbing.NewTagReferenceName(revision)
	checkoutOpts = &gogit.CheckoutOptions{
		Branch: tagRef,
		Force:  true,
	}
	err = wt.Checkout(checkoutOpts)
	if err == nil {
		return nil
	}

	// Try as commit hash
	hash := plumbing.NewHash(revision)
	checkoutOpts = &gogit.CheckoutOptions{
		Hash:  hash,
		Force: true,
	}
	if err := wt.Checkout(checkoutOpts); err != nil {
		return fmt.Errorf("checking out %q (tried branch, tag, commit): %w", revision, err)
	}

	return nil
}

// CheckoutInDir opens a repo at repoDir and checks out the given revision.
func CheckoutInDir(repoDir string, revision string) error {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("opening repo at %q: %w", repoDir, err)
	}
	return Checkout(repo, revision)
}

// CurrentCommit returns the current HEAD commit hash of the repository at repoDir.
func CurrentCommit(repoDir string) (string, error) {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return "", fmt.Errorf("opening repo at %q: %w", repoDir, err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("getting HEAD of repo at %q: %w", repoDir, err)
	}

	return ref.Hash().String(), nil
}

// IsCloned reports whether a git repository already exists at the given directory.
func IsCloned(repoDir string) bool {
	_, err := gogit.PlainOpen(repoDir)
	return err == nil
}
