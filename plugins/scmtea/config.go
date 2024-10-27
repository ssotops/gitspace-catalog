package main

import (
	"embed"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

//go:embed default-docker-compose.yaml
var defaultComposeFile embed.FS

const (
	pluginDataDir          = "/.ssot/gitspace/plugins/data/scmtea"
	composeFileName        = "docker-compose.yaml"
	defaultComposeFileName = "default-docker-compose.yaml"
)

func setComposeFile(p *ScmteaPlugin, option, customPath string) (*pb.CommandResponse, error) {
	dataDir := filepath.Join(os.Getenv("HOME"), pluginDataDir)
	destPath := filepath.Join(dataDir, composeFileName)

	p.logger.Info("Setting compose file", "dataDir", dataDir, "destPath", destPath)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		p.logger.Error("Failed to create plugin data directory", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Failed to create plugin data directory: %v", err),
		}, nil
	}

	switch option {
	case "Use default":
		p.logger.Info("Using default compose file")
		defaultCompose, err := defaultComposeFile.ReadFile(defaultComposeFileName)
		if err != nil {
			p.logger.Error("Failed to read default docker-compose.yaml", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read default docker-compose.yaml: %v", err),
			}, nil
		}
		if err = ioutil.WriteFile(destPath, defaultCompose, 0644); err != nil {
			p.logger.Error("Failed to write default docker-compose.yaml", "error", err)
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to write default docker-compose.yaml: %v", err),
			}, nil
		}
		p.logger.Info("Default compose file written successfully", "path", destPath)
	case "Enter custom path":
		if customPath == "" {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: "Custom path is required when choosing to enter a custom path",
			}, nil
		}
		if _, err := os.Stat(customPath); os.IsNotExist(err) {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("The specified docker-compose.yaml file does not exist: %s", customPath),
			}, nil
		}
		input, err := ioutil.ReadFile(customPath)
		if err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to read custom docker-compose.yaml: %v", err),
			}, nil
		}
		if err = ioutil.WriteFile(destPath, input, 0644); err != nil {
			return &pb.CommandResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("Failed to copy custom docker-compose.yaml: %v", err),
			}, nil
		}
	default:
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Invalid option selected",
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  fmt.Sprintf("Docker Compose file successfully set and copied to %s", destPath),
	}, nil
}

func getComposePath() (string, error) {
	dataDir := filepath.Join(os.Getenv("HOME"), pluginDataDir)
	composePath := filepath.Join(dataDir, composeFileName)
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return "", fmt.Errorf("docker-compose.yaml not found. Please use 'Set Docker Compose File' to set it")
	}
	return composePath, nil
}

func readDefaultValues() (DefaultValues, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultValues{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultsPath := filepath.Join(homeDir, ".ssot", "gitspace", "data", "scmtea", "defaults.toml")

	var defaults DefaultValues
	tree, err := toml.LoadFile(defaultsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file doesn't exist, return empty defaults
			return DefaultValues{}, nil
		}
		return DefaultValues{}, fmt.Errorf("failed to read defaults file: %w", err)
	}

	err = tree.Unmarshal(&defaults)
	if err != nil {
		return DefaultValues{}, fmt.Errorf("failed to unmarshal defaults: %w", err)
	}

	return defaults, nil
}

func updateDefaultValues(values DefaultValues) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	defaultsPath := filepath.Join(homeDir, ".ssot", "gitspace", "data", "scmtea", "defaults.toml")

	values.Gitea.LastUpdated = time.Now()

	f, err := os.Create(defaultsPath)
	if err != nil {
		return fmt.Errorf("failed to create defaults file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(values); err != nil {
		return fmt.Errorf("failed to encode defaults: %w", err)
	}

	return nil
}

func validateConfig(config map[string]interface{}) error {
	required := []string{"services", "version"}
	for _, field := range required {
		if _, ok := config[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	services, ok := config["services"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid services configuration")
	}

	if _, ok := services["gitea"]; !ok {
		return fmt.Errorf("missing gitea service configuration")
	}

	return nil
}

func ensureConfigDirectory(logger *logger.RateLimitedLogger) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, pluginDataDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return nil
}
