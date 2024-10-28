package main

import (
	"bufio"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/ssotops/gitspace-plugin-sdk/logger"
)

// Plugin structures
type ScmteaPlugin struct {
    logger *logger.RateLimitedLogger
}

// BackupConfig holds the configuration for backups
type BackupConfig struct {
    Schedule    string `toml:"backup_cron_expression"`
    S3Bucket    string `toml:"aws_s3_bucket_name"`
    S3Path      string `toml:"aws_s3_path"`
    AccessKey   string `toml:"aws_access_key_id"`
    SecretKey   string `toml:"aws_secret_access_key"`
    Endpoint    string `toml:"aws_endpoint"`
    LocalPath   string `toml:"backup_archive"`
    Retention   string `toml:"backup_retention_days"`
    Compression string `toml:"backup_compression"`
}

// DefaultValues represents default configuration values
type DefaultValues struct {
    Gitea struct {
        LastUpdated time.Time `toml:"last_updated"`
        Username    string    `toml:"username"`
        Password    string    `toml:"password"`
        Email       string    `toml:"email"`
    } `toml:"gitea"`
}

// Progress message structure
type ProgressMessage struct {
    Status  string `json:"status"`
    Step    string `json:"step"`
    Message string `json:"message"`
}

// bufferedWriteCloser wraps a buffered writer with mutex for thread safety
type bufferedWriteCloser struct {
    *bufio.Writer
    closer io.Closer
    mu     sync.Mutex
    closed bool
    logger *logger.RateLimitedLogger
}

// Plugin command response structures
type CommandResult struct {
    Success      bool   `json:"success"`
    Message      string `json:"message"`
    ErrorMessage string `json:"error_message,omitempty"`
}

// SSH key upload response
type SSHKeyUploadResponse struct {
    Success bool   `json:"success"`
    Status  string `json:"status"`
    Message string `json:"message"`
}

// Docker container information
type ContainerInfo struct {
    Name    string
    ID      string
    Status  string
    Running bool
}

// Git configuration information
type GitConfig struct {
    Name  string
    Email string
    Scope string
}

// Menu navigation state
type MenuState struct {
    CurrentMenu string
    ParentMenu  string
    Path        []string
}

// Plugin command
type Command struct {
    Name       string
    Parameters map[string]string
}

// Docker compose service
type ComposeService struct {
    Image       string   `yaml:"image"`
    Restart     string   `yaml:"restart"`
    Environment []string `yaml:"environment,omitempty"`
    Volumes     []string `yaml:"volumes,omitempty"`
    Labels      map[string]string `yaml:"labels,omitempty"`
}

// Compose configuration
type ComposeConfig struct {
    Version  string                      `yaml:"version"`
    Services map[string]ComposeService   `yaml:"services"`
    Volumes  map[string]map[string]any   `yaml:"volumes,omitempty"`
}

// Process information
type ProcessInfo struct {
    Command *exec.Cmd
    Stdin   io.WriteCloser
    Stdout  io.ReadCloser
    Stderr  io.ReadCloser
}

// Backup schedule information
type BackupSchedule struct {
    Expression string `json:"expression"`
    NextRun    string `json:"next_run"`
    LastRun    string `json:"last_run"`
    Status     string `json:"status"`
}

// Server status information
type ServerStatus struct {
    Running bool
    URL     string
    Port    int
    Version string
    Status  string
}

// Plugin metadata
type PluginMetadata struct {
    Name        string `toml:"name"`
    Version     string `toml:"version"`
    Description string `toml:"description"`
    Author      string `toml:"author"`
}
