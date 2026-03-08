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

// Pull fetches and pulls the latest changes in an already-cloned repository.
func Pull(projectCfg config.ProjectConfig, repoDir string) error {
	repo, err := gogit.PlainOpen(repoDir)
	if err != nil {
		return fmt.Errorf("opening repo at %q: %w", repoDir, err)
	}

	auth, err := authMethod(projectCfg.Git)
	if err != nil {
		return fmt.Errorf("resolving auth for project %q: %w", projectCfg.Name, err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting worktree: %w", err)
	}

	// Hard-reset to discard any local modifications (e.g. decrypted env files
	// written into the repo dir) so the pull cannot fail with "unstaged changes".
	if resetErr := wt.Reset(&gogit.ResetOptions{Mode: gogit.HardReset}); resetErr != nil {
		slog.Warn("hard reset before pull failed", "dir", repoDir, "error", resetErr)
	}

	pullOpts := &gogit.PullOptions{
		Auth:     auth,
		Progress: os.Stdout,
		Force:    true,
	}

	slog.Info("pulling repository", "dir", repoDir)
	err = wt.Pull(pullOpts)
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return fmt.Errorf("pulling %q: %w", repoDir, err)
	}

	if projectCfg.Git.Revision != "" {
		if err := Checkout(repo, projectCfg.Git.Revision); err != nil {
			return fmt.Errorf("checking out revision %q after pull: %w", projectCfg.Git.Revision, err)
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
