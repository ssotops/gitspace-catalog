package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

func commitAndPush(repoRoot string) error {
	if err := gitConfig(); err != nil {
		return fmt.Errorf("error configuring git: %w", err)
	}

	if err := gitAdd(repoRoot); err != nil {
		return fmt.Errorf("error adding files: %w", err)
	}

	if err := gitCommit(); err != nil {
		return fmt.Errorf("error committing changes: %w", err)
	}

	if err := gitPush(); err != nil {
		return fmt.Errorf("error pushing changes: %w", err)
	}

	fmt.Println("Changes committed and pushed successfully")
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

func gitCommit() error {
	cmd := exec.Command("git", "commit", "-m", "Update gitspace-catalog.toml")
	return cmd.Run()
}

func gitPush() error {
	cmd := exec.Command("git", "push")
	return cmd.Run()
}
