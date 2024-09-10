# Gitspace Catalog Automation

This directory contains the automation scripts for updating the Gitspace catalog.

## Structure

- `cmd/`: Contains Go scripts for the automation process
  - `dagger_pipeline.go`: Defines the Dagger pipeline
  - `update_catalog.go`: Updates the catalog TOML file
  - `commit_and_push.go`: Commits and pushes changes to the repository
- `workflows/`: Contains GitHub Actions workflow files
  - `update-catalog.yml`: Defines the workflow for updating the catalog

## Maintenance

To update the automation process:

1. Modify the Go scripts in the `cmd/` directory as needed
2. Update the `go.mod` file if new dependencies are added
3. Run `go mod tidy` in the `.github` directory to update `go.sum`
4. Test your changes locally before committing

## Running Locally

To run the Dagger pipeline locally:

```bash
cd .github
go run ./cmd/dagger_pipeline.go
```

Ensure you have the necessary environment variables set, particularly `GITHUB_TOKEN`.
