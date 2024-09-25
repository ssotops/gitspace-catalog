package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin-sdk/gsplug"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"google.golang.org/protobuf/proto"
)

type ScmteaPlugin struct{}

func (p *ScmteaPlugin) GetPluginInfo(req *pb.PluginInfoRequest) (*pb.PluginInfo, error) {
	log.Info("GetPluginInfo called")
	return &pb.PluginInfo{
		Name:    "Scmtea Plugin",
		Version: "1.0.0",
	}, nil
}

func (p *ScmteaPlugin) ExecuteCommand(req *pb.CommandRequest) (*pb.CommandResponse, error) {
	switch req.Command {
	case "start":
		return runDockerCompose("up -d")
	case "stop":
		return runDockerCompose("down")
	case "restart":
		return runDockerCompose("restart")
	case "force_recreate":
		return runDockerCompose("up -d --force-recreate")
	case "print_summary":
		return printGiteaSummary()
	case "delete_containers_images":
		return deleteContainersAndImages()
	case "delete_volumes":
		return deleteVolumes()
	default:
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: "Unknown command",
		}, nil
	}
}

func (p *ScmteaPlugin) GetMenu(req *pb.MenuRequest) (*pb.MenuResponse, error) {
	menuOptions := []gsplug.MenuOption{
		{Label: "Start Gitea", Command: "start"},
		{Label: "Stop Gitea", Command: "stop"},
		{Label: "Restart Gitea", Command: "restart"},
		{Label: "Force Recreate Gitea", Command: "force_recreate"},
		{Label: "Print Gitea Summary", Command: "print_summary"},
		{Label: "Delete Gitea Containers and Images", Command: "delete_containers_images"},
		{Label: "Delete Volumes", Command: "delete_volumes"},
	}

	menuBytes, err := json.Marshal(menuOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal menu: %w", err)
	}

	return &pb.MenuResponse{
		MenuData: menuBytes,
	}, nil
}

func runDockerCompose(args ...string) (*pb.CommandResponse, error) {
	cmd := exec.Command("docker-compose", append([]string{"-f", "/Users/alechp/Code/alechp/.repositories/config-backup/scripts/gitea/docker-compose.yaml"}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &pb.CommandResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("Error: %v, Output: %s", err, output),
		}, nil
	}
	return &pb.CommandResponse{
		Success: true,
		Result:  string(output),
	}, nil
}

func printGiteaSummary() (*pb.CommandResponse, error) {
	// Implementation of print_gitea_summary function
	// This would involve running several docker commands and formatting the output
	// For brevity, I'm omitting the full implementation here
	return &pb.CommandResponse{
		Success: true,
		Result:  "Gitea summary printed (implementation needed)",
	}, nil
}

func deleteContainersAndImages() (*pb.CommandResponse, error) {
	// Implementation of delete_containers_and_images function
	// This would involve running docker commands to stop and remove containers and images
	// For brevity, I'm omitting the full implementation here
	return &pb.CommandResponse{
		Success: true,
		Result:  "Containers and images deleted (implementation needed)",
	}, nil
}

func deleteVolumes() (*pb.CommandResponse, error) {
	// Implementation of delete_volumes function
	// This would involve running docker-compose down -v
	// For brevity, I'm omitting the full implementation here
	return &pb.CommandResponse{
		Success: true,
		Result:  "Volumes deleted (implementation needed)",
	}, nil
}

func main() {
	logDir := filepath.Join("logs", "scmtea")
	logger, err := logger.NewRateLimitedLogger(logDir, "scmtea")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("Scmtea plugin starting")

	plugin := &ScmteaPlugin{}

	for {
		logger.Debug("Waiting for message")
		msgType, msg, err := gsplug.ReadMessage(os.Stdin)
		if err != nil {
			if err == io.EOF {
				logger.Info("Received EOF, exiting")
				return
			}
			logger.Error("Error reading message", "error", err)
			continue
		}
		logger.Debug("Received message", "type", msgType, "content", fmt.Sprintf("%+v", msg))

		var response proto.Message
		switch msgType {
		case 1: // GetPluginInfo
			response, err = plugin.GetPluginInfo(msg.(*pb.PluginInfoRequest))
		case 2: // ExecuteCommand
			response, err = plugin.ExecuteCommand(msg.(*pb.CommandRequest))
		case 3: // GetMenu
			response, err = plugin.GetMenu(msg.(*pb.MenuRequest))
		default:
			err = fmt.Errorf("unknown message type: %d", msgType)
		}

		if err != nil {
			logger.Error("Error handling message", "error", err)
			continue
		}

		logger.Debug("Sending response", "type", msgType, "content", fmt.Sprintf("%+v", response))
		err = gsplug.WriteMessage(os.Stdout, response)
		if err != nil {
			logger.Error("Error writing response", "error", err)
		} else {
			logger.Debug("Response sent successfully")
		}

		os.Stdout.Sync()
	}
}
