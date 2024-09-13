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

	// determine the repository root (parent of .github)
	repoRoot := filepath.Dir(wd)
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

	// Verify catalog file exists and print its content
	content, err := os.ReadFile(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to read catalog file after update: %w", err)
	}
	fmt.Printf("Catalog file content after update:\n%s\n", string(content))

	// Get repository owner and name
	repoOwner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	repoName := os.Getenv("GITHUB_REPOSITORY")
	if repoOwner == "" || repoName == "" {
		return fmt.Errorf("GITHUB_REPOSITORY_OWNER or GITHUB_REPOSITORY environment variables are not set")
	}

	// If GITHUB_REPOSITORY includes the owner, split it
	if strings.Contains(repoName, "/") {
		parts := strings.SplitN(repoName, "/", 2)
		repoOwner = parts[0]
		repoName = parts[1]
	}

	// commit and push changes
	if err := commitAndPush(ctx, repoOwner, repoName, repoRoot); err != nil {
		return fmt.Errorf("failed to commit and push changes: %w", err)
	}

	fmt.Println("Catalog updated and changes pushed successfully")
	return nil
}
