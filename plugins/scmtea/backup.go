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

func (p *ScmteaPlugin) handleBackupCommands(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	switch req.Command {
	case "configure_backup":
		return p.configureBackup(req)
	case "create_backup":
		return p.createBackup()
	case "set_backup_schedule":
		return p.setBackupSchedule(req)
	case "restore_backup":
		return p.restoreFromBackup(req)
	default:
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Unknown backup command",
		}, nil
	}
}

func (p *ScmteaPlugin) configureBackup(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	// First ensure we have a valid compose file path and default compose file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get home directory: %v", err),
		}, nil
	}

	pluginDataDir := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea")
	composePath := filepath.Join(pluginDataDir, "docker-compose.yaml")

	// Ensure plugin data directory exists
	if err := os.MkdirAll(pluginDataDir, 0755); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create plugin data directory: %v", err),
		}, nil
	}

	// Check if compose file exists, if not copy default
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		defaultCompose, err := defaultComposeFile.ReadFile("default-docker-compose.yaml")
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read default compose file: %v", err),
			}, nil
		}

		if err := os.WriteFile(composePath, defaultCompose, 0644); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to write compose file: %v", err),
			}, nil
		}
	}

	// Create backup config
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

	// Read existing compose file
	content, err := os.ReadFile(composePath)
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to read compose file: %v", err),
		}, nil
	}

	// Parse existing compose file
	composeConfig := make(map[string]interface{})
	if err := yaml.Unmarshal(content, &composeConfig); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to parse compose file: %v", err),
		}, nil
	}

	// Initialize services map if it doesn't exist
	if composeConfig["services"] == nil {
		composeConfig["services"] = make(map[string]interface{})
	}
	services := composeConfig["services"].(map[string]interface{})

	// Add backup service
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
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to generate compose file: %v", err),
		}, nil
	}

	if err := os.WriteFile(composePath, updatedContent, 0644); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to write compose file: %v", err),
		}, nil
	}

	// Save backup config
	if err := saveBackupConfig(config); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to save backup config: %v", err),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Backup configuration updated successfully. Please restart Gitea to apply changes.",
	}, nil
}

func (p *ScmteaPlugin) createBackup() (*pb.CommandResponse, error) {
	// Trigger immediate backup by running the backup container with a custom command
	cmd := exec.Command("docker-compose", "run", "--rm", "backup", "/backup.sh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create backup: %v\nOutput: %s", err, output),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  fmt.Sprintf("Backup created successfully\nOutput: %s", output),
	}, nil
}

func (p *ScmteaPlugin) setBackupSchedule(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	schedule := req.Parameters["schedule"]
	if schedule == "" {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Backup schedule is required",
		}, nil
	}

	config, err := loadBackupConfig()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to load backup config: %v", err),
		}, nil
	}

	config.Schedule = schedule
	if err := saveBackupConfig(config); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to save backup config: %v", err),
		}, nil
	}

	// Update environment in compose file
	composePath, err := getComposePath()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to get compose path: %v", err),
		}, nil
	}

	content, err := ioutil.ReadFile(composePath)
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to read compose file: %v", err),
		}, nil
	}

	composeConfig := make(map[string]interface{})
	if err := yaml.Unmarshal(content, &composeConfig); err != nil {
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
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to generate compose file: %v", err),
		}, nil
	}

	if err := ioutil.WriteFile(composePath, updatedContent, 0644); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to write compose file: %v", err),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  fmt.Sprintf("Backup schedule set to: %s", schedule),
	}, nil
}

func (p *ScmteaPlugin) restoreFromBackup(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	backupFile := req.Parameters["backup_file"]
	if backupFile == "" {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Backup file path is required",
		}, nil
	}

	// Stop Gitea
	if _, err := runDockerCompose("down"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to stop Gitea: %v", err),
		}, nil
	}

	// Extract backup
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", "/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to extract backup: %v\nOutput: %s", err, output),
		}, nil
	}

	// Start Gitea
	if _, err := runDockerCompose("up", "-d"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to start Gitea: %v", err),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Backup restored successfully",
	}, nil
}

func saveBackupConfig(config BackupConfig) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "backup_config.toml")
	configDir := filepath.Dir(configPath)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(config)
}

func loadBackupConfig() (BackupConfig, error) {
	var config BackupConfig
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return config, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "backup_config.toml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil
	}

	tree, err := toml.LoadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := tree.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}
