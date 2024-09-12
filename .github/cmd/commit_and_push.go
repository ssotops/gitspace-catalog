package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
)

func commitAndPush(ctx context.Context, repoOwner, repoName string) error {
	appID, err := strconv.ParseInt(os.Getenv("APP_ID"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid APP_ID: %w", err)
	}

	installationID, err := strconv.ParseInt(os.Getenv("INSTALLATION_ID"), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid INSTALLATION_ID: %w", err)
	}

	privateKey := []byte(os.Getenv("APP_PRIVATE_KEY"))

	// Create a new transport using the GitHub App authentication
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return fmt.Errorf("error creating GitHub App transport: %w", err)
	}

	// Create a new GitHub client using the App authentication
	client := github.NewClient(&http.Client{Transport: itr})

	// Get the current commit SHA
	ref, _, err := client.Git.GetRef(ctx, repoOwner, repoName, "refs/heads/main")
	if err != nil {
		return fmt.Errorf("error getting ref: %w", err)
	}

	// Create a new tree with the updated catalog file
	tree, _, err := client.Git.CreateTree(ctx, repoOwner, repoName, *ref.Object.SHA, []*github.TreeEntry{
		{
			Path:    github.String("gitspace-catalog.toml"),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String("Your updated catalog content here"),
		},
	})
	if err != nil {
		return fmt.Errorf("error creating tree: %w", err)
	}

	// Create a new commit
	commit, _, err := client.Git.CreateCommit(ctx, repoOwner, repoName, &github.Commit{
		Message: github.String("Update gitspace-catalog.toml"),
		Tree:    tree,
		Parents: []*github.Commit{{SHA: ref.Object.SHA}},
	})
	if err != nil {
		return fmt.Errorf("error creating commit: %w", err)
	}

	// Update the reference
	_, _, err = client.Git.UpdateRef(ctx, repoOwner, repoName, &github.Reference{
		Ref:    github.String("refs/heads/main"),
		Object: &github.GitObject{SHA: commit.SHA},
	}, false)
	if err != nil {
		return fmt.Errorf("error updating ref: %w", err)
	}

	return nil
}

func gitConfig() error {
	cmd := exec.Command("git", "config", "--global", "user.email", "github-actions[bot]@users.noreply.github.com")
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--global", "user.name", "GitHub Actions")
	return cmd.Run()
}

func gitAdd(repoRoot string) error {
	cmd := exec.Command("git", "add", filepath.Join(repoRoot, "gitspace-catalog.toml"))
	cmd.Dir = repoRoot
	return cmd.Run()
}

func gitHasChanges(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

func gitCommit() error {
	cmd := exec.Command("git", "commit", "-m", "Update gitspace-catalog.toml")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

func gitPush() error {
	cmd := exec.Command("git", "push")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
