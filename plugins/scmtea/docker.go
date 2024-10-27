// docker.go

package main

import (
	"fmt"
	"os/exec"
	"strings"

	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

// Docker management functions all take *ScmteaPlugin to access logger

func checkDockerStatus(p *ScmteaPlugin) error {
	p.logger.Debug("Checking Docker daemon status")
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		p.logger.Error("Docker daemon check failed", "error", err)
		return fmt.Errorf("Docker is not running or accessible: %v", err)
	}
	p.logger.Debug("Docker daemon is running")
	return nil
}

func runDockerCompose(p *ScmteaPlugin, args ...string) (*pb.CommandResponse, error) {
	p.logger.Info("Running docker-compose command", "args", args)

	composePath, err := getComposePath()
	if err != nil {
		p.logger.Error("Error getting compose path", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}
	p.logger.Info("Compose file path", "path", composePath)

	cmdArgs := append([]string{"-f", composePath}, args...)
	cmd := exec.Command("docker-compose", cmdArgs...)
	p.logger.Info("Full docker-compose command", "command", cmd.String())
	output, err := cmd.CombinedOutput()
	p.logger.Info("docker-compose command output", "output", string(output))

	if err != nil {
		p.logger.Error("docker-compose command failed, attempting docker compose", "error", err)
		cmd = exec.Command("docker", append([]string{"compose", "-f", composePath}, args...)...)
		p.logger.Info("Full docker compose command", "command", cmd.String())
		output, err = cmd.CombinedOutput()
		p.logger.Info("docker compose command output", "output", string(output))
	}

	if err != nil {
		p.logger.Error("Error executing Docker Compose command", "error", err, "output", string(output))
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error executing Docker Compose command: %v\nOutput: %s", err, string(output)),
		}, nil
	}

	p.logger.Info("Docker Compose command executed successfully")
	return &pb.CommandResponse{
		Success: true,
		Result:  string(output),
	}, nil
}

func getRunningContainers(p *ScmteaPlugin) ([]string, error) {
	p.logger.Debug("Getting running Gitea containers")
	cmd := exec.Command("docker", "ps", "-q", "--filter", "name=gitea")
	output, err := cmd.CombinedOutput()
	if err != nil {
		p.logger.Error("Error listing running containers", "error", err)
		return nil, fmt.Errorf("Error listing running containers: %v", err)
	}
	containers := strings.Fields(string(output))
	p.logger.Debug("Found running containers", "count", len(containers))
	return containers, nil
}

func removeImages(p *ScmteaPlugin, imageName string) error {
	p.logger.Info("Removing Docker images", "image", imageName)
	images, err := exec.Command("docker", "images", imageName, "-q").Output()
	if err != nil {
		p.logger.Error("Error listing images", "image", imageName, "error", err)
		return fmt.Errorf("Error listing %s images: %v", imageName, err)
	}

	if len(images) > 0 {
		cmd := exec.Command("docker", "rmi", "-f", strings.TrimSpace(string(images)))
		cmdOutput, err := cmd.CombinedOutput()
		if err != nil {
			p.logger.Error("Error removing images",
				"image", imageName,
				"error", err,
				"output", string(cmdOutput))
			return fmt.Errorf("Error removing %s images: %v\nOutput: %s",
				imageName, err, cmdOutput)
		}
		p.logger.Info("Images removed successfully",
			"image", imageName,
			"output", string(cmdOutput))
	} else {
		p.logger.Info("No images found to remove", "image", imageName)
	}

	return nil
}

func deleteContainersAndImages(p *ScmteaPlugin) (*pb.CommandResponse, error) {
	p.logger.Info("Starting container and image cleanup")

	if err := checkDockerStatus(p); err != nil {
		p.logger.Error("Docker daemon is not running or accessible", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Docker daemon is not running or accessible: %v", err),
		}, nil
	}

	p.logger.Info("Stopping and removing containers")
	_, err := runDockerCompose(p, "down")
	if err != nil {
		p.logger.Error("Error stopping containers", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error stopping containers: %v", err),
		}, nil
	}

	runningContainers, err := getRunningContainers(p)
	if err != nil {
		p.logger.Error("Error checking for running containers", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error checking for running containers: %v", err),
		}, nil
	}

	if len(runningContainers) > 0 {
		p.logger.Warn("Containers still running, force stopping", "containers", runningContainers)
		for _, container := range runningContainers {
			cmdOutput, err := exec.Command("docker", "stop", container).CombinedOutput()
			if err != nil {
				p.logger.Error("Error force stopping container",
					"container", container,
					"error", err,
					"output", string(cmdOutput))
				return &pb.CommandResponse{
					Success: false,
					ErrorMessage: fmt.Sprintf("Error force stopping container %s: %v\nOutput: %s",
						container, err, cmdOutput),
				}, nil
			}
			p.logger.Info("Container force stopped",
				"container", container,
				"output", string(cmdOutput))
		}
	}

	// Remove Gitea images
	if err := removeImages(p, "gitea/gitea"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Remove Postgres images
	if err := removeImages(p, "postgres:13"); err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &pb.CommandResponse{
		Success: true,
		Result:  "Containers and images have been deleted.",
	}, nil
}

func deleteVolumes(p *ScmteaPlugin) (*pb.CommandResponse, error) {
	p.logger.Info("Deleting Docker volumes")
	_, err := runDockerCompose(p, "down", "-v")
	if err != nil {
		p.logger.Error("Error deleting volumes", "error", err)
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error deleting volumes: %v", err),
		}, nil
	}

	p.logger.Info("Volumes deleted successfully")
	return &pb.CommandResponse{
		Success: true,
		Result:  "Volumes have been deleted.",
	}, nil
}
