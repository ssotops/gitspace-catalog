# Initialize the Go module for the entire project
go mod init github.com/ssotops/gitspace-catalog

# Add dependencies for all programs
go get -d ./...

# Tidy up the go.mod file
go mod tidy
