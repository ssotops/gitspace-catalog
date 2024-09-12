package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
)

func main() {
	if err := build(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func build(ctx context.Context) error {
	fmt.Println("Building with Dagger")

	// initialize Dagger client
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	// get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// determine the repository root
	repoRoot := findRepoRoot(wd)
	fmt.Printf("Repository root: %s\n", repoRoot)

	// Check if gitspace-catalog.toml exists
	catalogPath := filepath.Join(repoRoot, "gitspace-catalog.toml")
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		return fmt.Errorf("gitspace-catalog.toml not found at %s", catalogPath)
	}

	// get reference to the local project
	src := client.Host().Directory(repoRoot)

	// get `golang` image
	golang := client.Container().From("golang:latest")

	// mount cloned repository into `golang` image
	golang = golang.WithDirectory("/src", src).WithWorkdir("/src")

	// update catalog
	if err := updateCatalog(repoRoot); err != nil {
		return fmt.Errorf("failed to update catalog: %w", err)
	}

	// commit and push changes
	repoOwner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	repoName := os.Getenv("GITHUB_REPOSITORY")
	if repoOwner == "" || repoName == "" {
		return fmt.Errorf("GITHUB_REPOSITORY_OWNER or GITHUB_REPOSITORY environment variables are not set")
	}
	if err := commitAndPush(ctx, repoOwner, repoName); err != nil {
		return fmt.Errorf("failed to commit and push changes: %w", err)
	}

	fmt.Println("Catalog updated and changes pushed successfully")
	return nil
}

func findRepoRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "gitspace-catalog.toml")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// We've reached the root
			if strings.HasSuffix(dir, "gitspace-catalog") {
				// We're likely in the GitHub Actions environment
				return dir
			}
			// If we can't find the root, return the starting directory
			return start
		}
		dir = parent
	}
}
