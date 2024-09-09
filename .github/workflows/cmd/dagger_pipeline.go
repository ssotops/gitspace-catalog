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
	src := client.Host().Directory(".")

	// create a directory for golang build cache
	cachedir := client.CacheVolume("gomodcache")

	// get `golang` image
	golang := client.Container().From("golang:latest")

	// mount cloned repository into `golang` image
	golang = golang.WithDirectory("/src", src).WithWorkdir("/src")

	// mount cache volume
	golang = golang.WithMountedCache("/root/.cache/go-build", cachedir)

	// define the application build command
	golang = golang.WithExec([]string{"go", "build", "-o", "update-catalog", "./cmd/update-catalog"})
	golang = golang.WithExec([]string{"go", "build", "-o", "commit-and-push", "./cmd/commit-and-push"})

	// run the update-catalog command
	golang = golang.WithExec([]string{"./update-catalog"})

	// run the commit-and-push command
	golang = golang.WithEnvVariable("GITHUB_TOKEN", os.Getenv("GITHUB_TOKEN"))
	golang = golang.WithExec([]string{"./commit-and-push"})

	// execute
	_, err = golang.Stdout(ctx)
	return err
}
