package main

import (
	"context"
	"fmt"
	"os"

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

	// get reference to the local project
	src := client.Host().Directory("../..")

	// get `golang` image
	golang := client.Container().From("golang:latest")

	// mount cloned repository into `golang` image
	golang = golang.WithDirectory("/src", src).WithWorkdir("/src")

	// define the application build command
	golang = golang.WithExec([]string{"go", "run", "./.github/cmd/update_catalog.go"})

	// run the commit-and-push command
	golang = golang.WithEnvVariable("GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN"))
	golang = golang.WithExec([]string{"go", "run", "./.github/cmd/commit_and_push.go"})

	// execute
	_, err = golang.Stdout(ctx)
	return err
}
