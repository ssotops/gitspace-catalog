package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/pelletier/go-toml"
    pb "github.com/ssotops/gitspace-plugin-sdk/proto"
    "gopkg.in/yaml.v3"
)

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

// Validates backup configuration parameters
func validateBackupConfig(params map[string]string) error {
    requiredParams := []string{
        "s3_bucket",
        "access_key",
        "secret_key",
        "endpoint",
    }

    for _, param := range requiredParams {
        if value, exists := params[param]; !exists || value == "" {
            return fmt.Errorf("missing required parameter: %s", param)
        }
    }

    return nil
}

// Handles all backup-related commands
func (p *ScmteaPlugin) handleBackupCommands(req *pb.CommandRequest) (*pb.CommandResponse, error) {
    p.logger.Info("Handling backup command", "command", req.Command)
    
    switch req.Command {
    case "configure_backup":
        return p.configureBackup(req)
    case "create_backup":
        return p.createBackup()
    case "set_backup_schedule":
        return p.setBackupSchedule(req)
    case "restore_backup":
        return p.restoreFromBackup(req)
    case "view_backup_summary":
        return p.getBackupSummary()
    default:
        p.logger.Error("Unknown backup command", "command", req.Command)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: "Unknown backup command",
        }, nil
    }
}

// Configures backup storage settings
func (p *ScmteaPlugin) configureBackup(req *pb.CommandRequest) (*pb.CommandResponse, error) {
    p.logger.Info("Configuring backup storage")
    
    config := BackupConfig{
        S3Bucket:    req.Parameters["s3_bucket"],
        S3Path:      req.Parameters["s3_path"],
        AccessKey:   req.Parameters["access_key"],
        SecretKey:   req.Parameters["secret_key"],
        Endpoint:    req.Parameters["endpoint"],
        LocalPath:   "/archive",
        Retention:   "7",
        Compression: "gz",
    }

    // Get compose path
    composePath, err := getComposePath()
    if err != nil {
        p.logger.Error("Failed to get compose path", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to get compose path: %v", err),
        }, nil
    }

    // Read existing compose file
    content, err := ioutil.ReadFile(composePath)
    if err != nil {
        p.logger.Error("Failed to read compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to read compose file: %v", err),
        }, nil
    }

    // Parse existing compose file
    composeConfig := make(map[string]interface{})
    if err := yaml.Unmarshal(content, &composeConfig); err != nil {
        p.logger.Error("Failed to parse compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to parse compose file: %v", err),
        }, nil
    }

    // Add backup service
    services := composeConfig["services"].(map[string]interface{})
    services["backup"] = map[string]interface{}{
        "image":   "offen/docker-volume-backup:latest",
        "restart": "always",
        "environment": []string{
            fmt.Sprintf("AWS_S3_BUCKET_NAME=%s", config.S3Bucket),
            fmt.Sprintf("AWS_S3_PATH=%s", config.S3Path),
            fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", config.AccessKey),
            fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", config.SecretKey),
            fmt.Sprintf("AWS_ENDPOINT=%s", config.Endpoint),
            fmt.Sprintf("BACKUP_RETENTION_DAYS=%s", config.Retention),
            fmt.Sprintf("BACKUP_COMPRESSION=%s", config.Compression),
            "BACKUP_FILENAME=gitea-backup-%Y-%m-%dT%H-%M-%S.tar.gz",
        },
        "volumes": []string{
            "gitea_data:/backup/gitea:ro",
            "/var/run/docker.sock:/var/run/docker.sock:ro",
            fmt.Sprintf("%s:/archive", config.LocalPath),
        },
    }

    // Add label to Gitea service for stopping during backup
    if giteaService, ok := services["gitea"].(map[string]interface{}); ok {
        if labels, ok := giteaService["labels"].(map[string]interface{}); ok {
            labels["docker-volume-backup.stop-during-backup"] = "true"
        } else {
            giteaService["labels"] = map[string]interface{}{
                "docker-volume-backup.stop-during-backup": "true",
            }
        }
    }

    // Save updated compose file
    updatedContent, err := yaml.Marshal(composeConfig)
    if err != nil {
        p.logger.Error("Failed to generate compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to generate compose file: %v", err),
        }, nil
    }

    if err := ioutil.WriteFile(composePath, updatedContent, 0644); err != nil {
        p.logger.Error("Failed to write compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to write compose file: %v", err),
        }, nil
    }

    // Save backup config
    if err := p.saveBackupConfig(config); err != nil {
        p.logger.Error("Failed to save backup config", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to save backup config: %v", err),
        }, nil
    }

    p.logger.Info("Backup configuration updated successfully")
    return &pb.CommandResponse{
        Success: true,
        Result:  "Backup configuration updated successfully. Please restart Gitea to apply changes.",
    }, nil
}

// Creates an immediate backup
func (p *ScmteaPlugin) createBackup() (*pb.CommandResponse, error) {
    p.logger.Info("Starting immediate backup")
    cmd := exec.Command("docker-compose", "run", "--rm", "backup", "/backup.sh")
    output, err := cmd.CombinedOutput()
    if err != nil {
        p.logger.Error("Backup failed", "error", err, "output", string(output))
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to create backup: %v\nOutput: %s", err, output),
        }, nil
    }

    p.logger.Info("Backup completed successfully")
    return &pb.CommandResponse{
        Success: true,
        Result:  fmt.Sprintf("Backup created successfully\nOutput: %s", output),
    }, nil
}

// Sets the backup schedule
func (p *ScmteaPlugin) setBackupSchedule(req *pb.CommandRequest) (*pb.CommandResponse, error) {
    p.logger.Info("Setting backup schedule")
    schedule := req.Parameters["schedule"]
    if schedule == "" {
        p.logger.Error("Backup schedule is required")
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: "Backup schedule is required",
        }, nil
    }

    config, err := p.loadBackupConfig()
    if err != nil {
        p.logger.Error("Failed to load backup config", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to load backup config: %v", err),
        }, nil
    }

    config.Schedule = schedule
    if err := p.saveBackupConfig(config); err != nil {
        p.logger.Error("Failed to save backup config", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to save backup config: %v", err),
        }, nil
    }

    // Update environment in compose file
    composePath, err := getComposePath()
    if err != nil {
        p.logger.Error("Failed to get compose path", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to get compose path: %v", err),
        }, nil
    }

    content, err := ioutil.ReadFile(composePath)
    if err != nil {
        p.logger.Error("Failed to read compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to read compose file: %v", err),
        }, nil
    }

    composeConfig := make(map[string]interface{})
    if err := yaml.Unmarshal(content, &composeConfig); err != nil {
        p.logger.Error("Failed to parse compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to parse compose file: %v", err),
        }, nil
    }

    services := composeConfig["services"].(map[string]interface{})
    if backupService, ok := services["backup"].(map[string]interface{}); ok {
        if env, ok := backupService["environment"].([]string); ok {
            newEnv := []string{}
            for _, e := range env {
                if !strings.HasPrefix(e, "BACKUP_CRON_EXPRESSION=") {
                    newEnv = append(newEnv, e)
                }
            }
            newEnv = append(newEnv, fmt.Sprintf("BACKUP_CRON_EXPRESSION=%s", schedule))
            backupService["environment"] = newEnv
        }
    }

    updatedContent, err := yaml.Marshal(composeConfig)
    if err != nil {
        p.logger.Error("Failed to generate compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to generate compose file: %v", err),
        }, nil
    }

    if err := ioutil.WriteFile(composePath, updatedContent, 0644); err != nil {
        p.logger.Error("Failed to write compose file", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to write compose file: %v", err),
        }, nil
    }

    p.logger.Info("Backup schedule updated successfully", "schedule", schedule)
    return &pb.CommandResponse{
        Success: true,
        Result:  fmt.Sprintf("Backup schedule set to: %s", schedule),
    }, nil
}

// Restores from a backup file
func (p *ScmteaPlugin) restoreFromBackup(req *pb.CommandRequest) (*pb.CommandResponse, error) {
    p.logger.Info("Starting backup restore")
    backupFile := req.Parameters["backup_file"]
    if backupFile == "" {
        p.logger.Error("Backup file path is required")
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: "Backup file path is required",
        }, nil
    }

    // Stop Gitea
    p.logger.Info("Stopping Gitea for restore")
    if _, err := runDockerCompose("down"); err != nil {
        p.logger.Error("Failed to stop Gitea", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to stop Gitea: %v", err),
        }, nil
    }

    // Extract backup
    p.logger.Info("Extracting backup file", "file", backupFile)
    cmd := exec.Command("tar", "-xzf", backupFile, "-C", "/")
    output, err := cmd.CombinedOutput()
    if err != nil {
        p.logger.Error("Failed to extract backup", "error", err, "output", string(output))
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to extract backup: %v\nOutput: %s", err, output),
        }, nil
    }

    // Start Gitea
    p.logger.Info("Starting Gitea after restore")
    if _, err := runDockerCompose("up", "-d"); err != nil {
        p.logger.Error("Failed to start Gitea", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to start Gitea: %v", err),
        }, nil
    }

    p.logger.Info("Backup restored successfully")
    return &pb.CommandResponse{
        Success: true,
        Result:  "Backup restored successfully",
    }, nil
}

// Gets backup configuration summary
func (p *ScmteaPlugin) getBackupSummary() (*pb.CommandResponse, error) {
    p.logger.Info("Getting backup configuration summary")
    config, err := p.loadBackupConfig()
    if err != nil {
        p.logger.Error("Failed to load backup config", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to load backup config: %v", err),
        }, nil
    }

    summary := fmt.Sprintf(`Backup Configuration:
S3 Bucket: %s
S3 Path: %s
Endpoint: %s
Schedule: %s
Retention: %s days
Compression: %s
Local Path: %s`,
        config.S3Bucket,
        config.S3Path,
        config.Endpoint,
        config.Schedule,
        config.Retention,
        config.Compression,
        config.LocalPath)

    return &pb.CommandResponse{
        Success: true,
        Result:  summary,
    }, nil
}

// Helper functions for backup configuration persistence
func (p *ScmteaPlugin) saveBackupConfig(config BackupConfig) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        p.logger.Error("Failed to get home directory", "error", err)
        return fmt.Errorf("failed to get home directory: %w", err)
    }

    configPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "backup_config.toml")
    configDir := filepath.Dir(configPath)

    if err := os.MkdirAll(configDir, 0755); err != nil {
        p.logger.Error("Failed to create config directory", "error", err)
        return fmt.Errorf("failed to create config directory: %w", err)
    }

    file, err := os.Create(configPath)
    if err != nil {
        p.logger.Error("Failed to create config file", "error", err)
        return fmt.Errorf("failed to create config file: %w", err)
    }
    defer file.Close()

    encoder := toml.NewEncoder(file)
    if err := encoder.Encode(config); err != nil {
        p.logger.Error("Failed to encode config", "error", err)
        return fmt.Errorf("failed to encode config: %w", err)
    }

    p.logger.Info("Backup configuration saved successfully", "path", configPath)
    return nil
}

func (p *ScmteaPlugin) loadBackupConfig() (BackupConfig, error) {
    var config BackupConfig
    homeDir, err := os.UserHomeDir()
    if err != nil {
        p.logger.Error("Failed to get home directory", "error", err)
        return config, fmt.Errorf("failed to get home directory: %w", err)
    }

    configPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "backup_config.toml")
    
    if _, err := os.Stat(configPath); os.IsNotExist(err) {
        p.logger.Info("No existing backup configuration found", "path", configPath)
        return config, nil
    }

    tree, err := toml.LoadFile(configPath)
    if err != nil {
        p.logger.Error("Failed to read config file", "error", err)
        return config, fmt.Errorf("failed to read config file: %w", err)
    }

    if err := tree.Unmarshal(&config); err != nil {
        p.logger.Error("Failed to parse config file", "error", err)
        return config, fmt.Errorf("failed to parse config file: %w", err)
    }

    p.logger.Info("Backup configuration loaded successfully")
    return config, nil
}
