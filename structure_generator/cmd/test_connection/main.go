package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	log.Println("Creating MCP client...")

	// Create client
	mcpClient, err := client.NewStdioMCPClient("npx", []string{}, "-y", "@modelcontextprotocol/server-everything")
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer mcpClient.Close()

	log.Println("Client created, initializing...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "test",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	log.Println("Sending initialize request...")
	initResp, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}

	log.Printf("Initialize succeeded! Server: %s %s\n", initResp.ServerInfo.Name, initResp.ServerInfo.Version)

	// List tools
	log.Println("Listing tools...")
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		log.Fatalf("ListTools failed: %v", err)
	}

	log.Printf("Success! Found %d tools:\n", len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		fmt.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}
}
