// ssh.go

package main

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

func generateAndUploadSSHKey(p *ScmteaPlugin, req *pb.CommandRequest) (*pb.CommandResponse, error) {
    username := req.Parameters["username"]
    password := req.Parameters["password"]
    email := req.Parameters["email"]

    if username == "" || password == "" || email == "" {
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: "Missing required parameters: username, password, and email are required",
        }, nil
    }

    sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
    if err := os.MkdirAll(sshDir, 0700); err != nil {
        p.logger.Error("Failed to create .ssh directory", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Error creating .ssh directory: %v", err),
        }, nil
    }

    uniqueID, err := generateUniqueID()
    if err != nil {
        p.logger.Error("Failed to generate unique ID", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Error generating unique ID: %v", err),
        }, nil
    }

    sshKeyName := fmt.Sprintf("id_ed25519_gitea_%s_%s", username, uniqueID)
    sshKeyPath := filepath.Join(sshDir, sshKeyName)

    p.logger.Info("Generating SSH key", "path", sshKeyPath)
    cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-C", email, "-f", sshKeyPath, "-N", "")
    output, err := cmd.CombinedOutput()
    if err != nil {
        p.logger.Error("Failed to generate SSH key", "error", err, "output", string(output))
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Error generating SSH key: %v\nOutput: %s", err, output),
        }, nil
    }

    pubKeyBytes, err := ioutil.ReadFile(sshKeyPath + ".pub")
    if err != nil {
        p.logger.Error("Failed to read public key", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Error reading public key: %v", err),
        }, nil
    }
    pubKey := string(pubKeyBytes)

    homeDir, err := os.UserHomeDir()
    if err != nil {
        p.logger.Error("Failed to get user home directory", "error", err)
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to get user home directory: %v", err),
        }, nil
    }

    uploadScriptPath := filepath.Join(homeDir, ".ssot", "gitspace", "plugins", "data", "scmtea", "ssh-key", "index.js")

    p.logger.Info("Running SSH key upload script...")
    cmd = exec.Command("bun", "run", uploadScriptPath, username, password, pubKey)
    cmdOutput, err := cmd.CombinedOutput()
    if err != nil {
        p.logger.Error("SSH key upload script failed", "error", err, "output", string(cmdOutput))
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("SSH key upload script failed: %v\nOutput: %s", err, cmdOutput),
        }, nil
    }

    var result struct {
        Success bool   `json:"success"`
        Status  string `json:"status"`
        Message string `json:"message"`
    }

    if err := json.Unmarshal(cmdOutput, &result); err != nil {
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("Failed to parse script output: %v\nOutput: %s", err, cmdOutput),
        }, nil
    }

    if !result.Success {
        return &pb.CommandResponse{
            Success:      false,
            ErrorMessage: fmt.Sprintf("SSH key upload failed: %s", result.Message),
        }, nil
    }

    return &pb.CommandResponse{
        Success: true,
        Result:  fmt.Sprintf("SSH key generated and uploaded successfully. Private key path: %s", sshKeyPath),
    }, nil
}

func generateUniqueID() (string, error) {
    randomBytes := make([]byte, 4)
    if _, err := rand.Read(randomBytes); err != nil {
        return "", err
    }
    timestamp := time.Now().Unix()
    combined := append([]byte(fmt.Sprintf("%d", timestamp)), randomBytes...)
    return hex.EncodeToString(combined), nil
}

func validateSSHKey(p *ScmteaPlugin, keyPath string) error {
    p.logger.Debug("Validating SSH key", "path", keyPath)
    
    info, err := os.Stat(keyPath)
    if err != nil {
        if os.IsNotExist(err) {
            p.logger.Error("SSH key not found", "path", keyPath)
            return fmt.Errorf("SSH key not found at %s", keyPath)
        }
        p.logger.Error("Error accessing SSH key", "error", err)
        return fmt.Errorf("error accessing SSH key: %v", err)
    }

    // Check permissions
    mode := info.Mode()
    if mode&0077 != 0 {
        p.logger.Error("SSH key has incorrect permissions", 
            "path", keyPath,
            "mode", fmt.Sprintf("%o", mode))
        return fmt.Errorf("SSH key has incorrect permissions. Should be 0600, got %o", mode)
    }

    p.logger.Info("SSH key validation successful", "path", keyPath)
    return nil
}

func configureSSHConfig(p *ScmteaPlugin, keyPath string) error {
    p.logger.Info("Configuring SSH config for Gitea")
    
    sshConfigPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
    
    // Create config file if it doesn't exist
    if _, err := os.Stat(sshConfigPath); os.IsNotExist(err) {
        p.logger.Debug("SSH config file does not exist, creating it")
        if err := ioutil.WriteFile(sshConfigPath, []byte(""), 0600); err != nil {
            p.logger.Error("Failed to create SSH config file", "error", err)
            return fmt.Errorf("failed to create SSH config file: %v", err)
        }
    }

    content, err := ioutil.ReadFile(sshConfigPath)
    if err != nil {
        p.logger.Error("Failed to read SSH config", "error", err)
        return fmt.Errorf("failed to read SSH config: %v", err)
    }

    // Check if there's already a Gitea host entry
    if !strings.Contains(string(content), "Host gitea") {
        p.logger.Debug("Adding Gitea host configuration")
        newConfig := fmt.Sprintf(`
Host gitea
    HostName localhost
    User git
    Port 22
    IdentityFile %s
`, keyPath)

        if err := ioutil.WriteFile(sshConfigPath, []byte(string(content)+newConfig), 0600); err != nil {
            p.logger.Error("Failed to update SSH config", "error", err)
            return fmt.Errorf("failed to update SSH config: %v", err)
        }
        p.logger.Info("SSH config updated successfully")
    } else {
        p.logger.Info("Gitea host configuration already exists in SSH config")
    }

    return nil
}
