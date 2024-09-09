package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if err := gitConfig(); err != nil {
		fmt.Println("Error configuring git:", err)
		os.Exit(1)
	}

	if err := gitAdd(); err != nil {
		fmt.Println("Error adding files:", err)
		os.Exit(1)
	}

	if err := gitCommit(); err != nil {
		fmt.Println("Error committing changes:", err)
		os.Exit(1)
	}

	if err := gitPush(); err != nil {
		fmt.Println("Error pushing changes:", err)
		os.Exit(1)
	}

	fmt.Println("Changes committed and pushed successfully")
}

func gitConfig() error {
	cmd := exec.Command("git", "config", "--global", "user.email", "github-actions[bot]@users.noreply.github.com")
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--global", "user.name", "GitHub Actions")
	return cmd.Run()
}

func gitAdd() error {
	cmd := exec.Command("git", "add", "gitspace-catalog.toml")
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
