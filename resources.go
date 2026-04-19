// resources.go - MCP resource definitions
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// configureResources adds all resources to the MCP server
func configureResources(server *mcp.Server, client *SlackClient) {
	// Resource: slack://channels - List of accessible channels
	server.AddResource(&mcp.Resource{
		URI:         "slack://channels",
		Name:        "channels",
		Description: "List of accessible Slack channels and conversations",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		channels, err := client.ListConversations("public_channel,private_channel,mpim,im", 100)
		if err != nil {
			return nil, fmt.Errorf("failed to list channels: %w", err)
		}

		data, err := json.Marshal(channels)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal channels: %w", err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      "slack://channels",
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	})

	// Resource: slack://channel/{id}/messages - Recent messages for a channel
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "slack://channel/{id}/messages",
		Name:         "channel-messages",
		Description:  "Recent messages from a specific Slack channel",
		MIMEType:     "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		// Extract channel ID from URI
		channelID := extractChannelIDFromURI(req.Params.URI)
		if channelID == "" {
			return nil, fmt.Errorf("invalid URI format: %s", req.Params.URI)
		}

		result, err := client.GetConversationHistory(channelID, 50, "", "")
		if err != nil {
			return nil, fmt.Errorf("failed to get messages: %w", err)
		}

		data, err := json.Marshal(result.Messages)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal messages: %w", err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      req.Params.URI,
					MIMEType: "application/json",
					Text:     string(data),
				},
			},
		}, nil
	})
}

// extractChannelIDFromURI extracts the channel ID from a resource URI
// Expected format: slack://channel/{id}/messages
func extractChannelIDFromURI(uri string) string {
	prefix := "slack://channel/"
	suffix := "/messages"

	if !strings.HasPrefix(uri, prefix) || !strings.HasSuffix(uri, suffix) {
		return ""
	}

	// Extract the ID between prefix and suffix
	start := len(prefix)
	end := len(uri) - len(suffix)
	if start >= end {
		return ""
	}

	return uri[start:end]
}