package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// func gitConfig() error {
// 	cmd := exec.Command("git", "config", "--global", "user.email", "github-actions[bot]@users.noreply.github.com")
// 	if err := cmd.Run(); err != nil {
// 		return err
// 	}

// 	cmd = exec.Command("git", "config", "--global", "user.name", "GitHub Actions")
// 	return cmd.Run()
// }

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
