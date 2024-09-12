package main

import (
	"context"
	"fmt"
  "io/ioutil"
	"net/http"
	"os"
	"strconv"

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

	// Log the repository information
	fmt.Printf("Attempting to access repository: %s/%s\n", repoOwner, repoName)

	// Get the current commit SHA
	ref, _, err := client.Git.GetRef(ctx, repoOwner, repoName, "heads/master")
	if err != nil {
		// Log more details about the error
		fmt.Printf("Error getting ref: %v\n", err)
		if errResp, ok := err.(*github.ErrorResponse); ok {
			fmt.Printf("GitHub API responded with status: %s\n", errResp.Response.Status)
			fmt.Printf("GitHub API error message: %s\n", errResp.Message)
		}
		return fmt.Errorf("error getting ref: %w", err)
	}

	// Read the updated catalog content
	catalogContent, err := ioutil.ReadFile("gitspace-catalog.toml")
	if err != nil {
		return fmt.Errorf("error reading updated catalog file: %w", err)
	}

	// Create a new tree with the updated catalog file
	tree, _, err := client.Git.CreateTree(ctx, repoOwner, repoName, *ref.Object.SHA, []*github.TreeEntry{
		{
			Path:    github.String("gitspace-catalog.toml"),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(string(catalogContent)),
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
		Ref:    github.String("refs/heads/master"),
		Object: &github.GitObject{SHA: commit.SHA},
	}, false)
	if err != nil {
		return fmt.Errorf("error updating ref: %w", err)
	}

	fmt.Println("Successfully committed and pushed changes")
	return nil
}
