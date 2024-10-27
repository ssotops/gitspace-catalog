package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"google.golang.org/protobuf/proto"
)

func main() {
	// Set up logger
	pluginLogger, err := logger.NewRateLimitedLogger("scmtea")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	pluginLogger.SetLogLevel(log.DebugLevel)

	pluginLogger.Info("Scmtea plugin starting up")
	dir, err := os.Getwd()
	if err != nil {
		dir = "unknown"
	}
	pluginLogger.Debug("Process information",
		"pid", os.Getpid(),
		"ppid", os.Getppid(),
		"uid", os.Getuid(),
		"gid", os.Getgid(),
		"dir", dir)

	plugin := &ScmteaPlugin{
		logger: pluginLogger,
	}

	// Set up signal handling with logging
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create error channel
	errChan := make(chan error, 1)

	// Start the main plugin loop in a goroutine
	go func() {
		reader := bufio.NewReader(os.Stdin)
		writer := bufio.NewWriter(os.Stdout)

		pluginLogger.Debug("Created IO buffers",
			"readerSize", reader.Size(),
			"writerSize", writer.Size())

		for {
			// Read message type
			pluginLogger.Debug("Waiting to read message type")
			msgTypeByte := make([]byte, 1)
			n, err := io.ReadFull(reader, msgTypeByte)
			if err != nil {
				if err == io.EOF {
					pluginLogger.Info("Received EOF, exiting normally")
					errChan <- nil
					return
				}
				errChan <- fmt.Errorf("failed to read message type: %w", err)
				return
			}
			pluginLogger.Debug("Read message type byte",
				"bytesRead", n,
				"messageType", msgTypeByte[0])

			// Read message length
			pluginLogger.Debug("Reading message length")
			var msgLen uint32
			if err := binary.Read(reader, binary.LittleEndian, &msgLen); err != nil {
				errChan <- fmt.Errorf("failed to read message length: %w", err)
				return
			}
			pluginLogger.Debug("Read message length", "length", msgLen)

			// Read message data
			data := make([]byte, msgLen)
			n, err = io.ReadFull(reader, data)
			if err != nil {
				errChan <- fmt.Errorf("failed to read message data: %w", err)
				return
			}
			pluginLogger.Debug("Read message data",
				"bytesRead", n,
				"dataLength", len(data),
				"data", fmt.Sprintf("%x", data))

			// Handle the message
			var response proto.Message
			msgType := uint32(msgTypeByte[0])
			switch msgType {
			case 1: // GetPluginInfo
				pluginLogger.Debug("Handling GetPluginInfo request")
				req := &pb.PluginInfoRequest{}
				if err := proto.Unmarshal(data, req); err != nil {
					errChan <- fmt.Errorf("failed to unmarshal GetPluginInfo request: %w", err)
					return
				}
				response, err = plugin.GetPluginInfo(req)

			case 2: // ExecuteCommand
				pluginLogger.Debug("Handling ExecuteCommand request")
				req := &pb.CommandRequest{}
				if err := proto.Unmarshal(data, req); err != nil {
					errChan <- fmt.Errorf("failed to unmarshal ExecuteCommand request: %w", err)
					return
				}
				response, err = plugin.ExecuteCommand(req)

			case 3: // GetMenu
				pluginLogger.Debug("Handling GetMenu request")
				req := &pb.MenuRequest{}
				if err := proto.Unmarshal(data, req); err != nil {
					errChan <- fmt.Errorf("failed to unmarshal GetMenu request: %w", err)
					return
				}
				response, err = plugin.GetMenu(req)

			default:
				errChan <- fmt.Errorf("unknown message type: %d", msgType)
				return
			}

			if err != nil {
				errChan <- fmt.Errorf("error handling message type %d: %w", msgType, err)
				return
			}

			// Marshal and send response
			pluginLogger.Debug("Marshaling response",
				"type", fmt.Sprintf("%T", response))
			responseData, err := proto.Marshal(response)
			if err != nil {
				errChan <- fmt.Errorf("failed to marshal response: %w", err)
				return
			}

			// Write response type
			pluginLogger.Debug("Writing response type", "type", msgType)
			if _, err := writer.Write([]byte{byte(msgType)}); err != nil {
				errChan <- fmt.Errorf("failed to write response type: %w", err)
				return
			}

			// Write response length
			pluginLogger.Debug("Writing response length", "length", len(responseData))
			if err := binary.Write(writer, binary.LittleEndian, uint32(len(responseData))); err != nil {
				errChan <- fmt.Errorf("failed to write response length: %w", err)
				return
			}

			// Write response data
			pluginLogger.Debug("Writing response data",
				"dataLength", len(responseData),
				"data", fmt.Sprintf("%x", responseData))
			if _, err := writer.Write(responseData); err != nil {
				errChan <- fmt.Errorf("failed to write response data: %w", err)
				return
			}

			// Flush the writer
			pluginLogger.Debug("Flushing writer")
			if err := writer.Flush(); err != nil {
				errChan <- fmt.Errorf("failed to flush writer: %w", err)
				return
			}
			pluginLogger.Debug("Response sent successfully")
		}
	}()

	// Wait for either an error or a signal
	select {
	case err := <-errChan:
		if err != nil {
			pluginLogger.Error("Plugin error",
				"error", err,
				"errorType", fmt.Sprintf("%T", err))
			os.Exit(1)
		}
		pluginLogger.Info("Plugin exiting normally")
	case sig := <-sigChan:
		pluginLogger.Info("Received signal, shutting down",
			"signal", sig,
			"signalType", fmt.Sprintf("%T", sig))
	}
}
