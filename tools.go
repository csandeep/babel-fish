// tools.go - MCP tool definitions and handlers
package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ToolHandler holds the Slack client and implements MCP tools
type ToolHandler struct {
	client *SlackClient
}

// NewToolHandler creates a new tool handler
func NewToolHandler(client *SlackClient) *ToolHandler {
	return &ToolHandler{client: client}
}

// Input types for each tool

type readMessagesInput struct {
	ChannelID string `json:"channel_id"`
	Limit     int    `json:"limit"`
	Oldest    string `json:"oldest"`
	Latest    string `json:"latest"`
}

type readThreadInput struct {
	ChannelID string `json:"channel_id"`
	ThreadTS  string `json:"thread_ts"`
	Limit     int    `json:"limit"`
}

type listChannelsInput struct {
	Types string `json:"types"`
	Limit int    `json:"limit"`
}

type searchMessagesInput struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

type getPermalinkInput struct {
	URL string `json:"url"`
}

// Output type (all tools return text)
type textOutput struct {
	Text string `json:"text"`
}

// configureTools adds all tools to the MCP server using the typed AddTool pattern
func configureTools(server *mcp.Server, handler *ToolHandler) {
	// Tool: slack_read_messages
	mcp.AddTool(server, &mcp.Tool{
		Name:        "slack_read_messages",
		Description: "Read messages from a Slack channel or conversation",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input readMessagesInput) (*mcp.CallToolResult, textOutput, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 50
		}
		result, err := handler.client.GetConversationHistory(input.ChannelID, limit, input.Oldest, input.Latest)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error fetching messages: %v", err)}},
				IsError: true,
			}, textOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatMessages(result.Messages)}},
		}, textOutput{}, nil
	})

	// Tool: slack_read_thread
	mcp.AddTool(server, &mcp.Tool{
		Name:        "slack_read_thread",
		Description: "Read replies in a Slack thread",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input readThreadInput) (*mcp.CallToolResult, textOutput, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 50
		}
		result, err := handler.client.GetConversationReplies(input.ChannelID, input.ThreadTS, limit)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error fetching thread: %v", err)}},
				IsError: true,
			}, textOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatMessages(result.Messages)}},
		}, textOutput{}, nil
	})

	// Tool: slack_list_channels
	mcp.AddTool(server, &mcp.Tool{
		Name:        "slack_list_channels",
		Description: "List accessible Slack channels and conversations",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input listChannelsInput) (*mcp.CallToolResult, textOutput, error) {
		limit := input.Limit
		if limit <= 0 {
			limit = 100
		}
		channels, err := handler.client.ListConversations(input.Types, limit)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error listing channels: %v", err)}},
				IsError: true,
			}, textOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatChannels(channels)}},
		}, textOutput{}, nil
	})

	// Tool: slack_search_messages
	mcp.AddTool(server, &mcp.Tool{
		Name:        "slack_search_messages",
		Description: "Search messages across Slack workspace",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input searchMessagesInput) (*mcp.CallToolResult, textOutput, error) {
		count := input.Count
		if count <= 0 {
			count = 20
		}
		result, err := handler.client.SearchMessages(input.Query, count)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error searching messages: %v", err)}},
				IsError: true,
			}, textOutput{}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatSearchResults(result)}},
		}, textOutput{}, nil
	})

	// Tool: slack_get_permalink
	mcp.AddTool(server, &mcp.Tool{
		Name:        "slack_get_permalink",
		Description: "Parse a Slack message permalink and extract channel_id and timestamp",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getPermalinkInput) (*mcp.CallToolResult, textOutput, error) {
		channelID, timestamp := parseSlackPermalink(input.URL)
		if channelID == "" || timestamp == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Could not parse permalink. Expected format: https://<workspace>.slack.com/archives/<channel_id>/p<timestamp>"}},
				IsError: true,
			}, textOutput{}, nil
		}
		text := fmt.Sprintf("Parsed permalink:\n- Channel ID: %s\n- Timestamp: %s\n\nTo read this message, use slack_read_messages with:\n{\"channel_id\": \"%s\", \"latest\": \"%s\", \"limit\": 1}\n\nOr to read thread replies, use slack_read_thread with:\n{\"channel_id\": \"%s\", \"thread_ts\": \"%s\"}", channelID, timestamp, channelID, timestamp, channelID, timestamp)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, textOutput{}, nil
	})
}

// parseSlackPermalink extracts channel_id and timestamp from a Slack URL
func parseSlackPermalink(rawURL string) (channelID, timestamp string) {
	// Pattern: https://<workspace>.slack.com/archives/<channel_id>/p<timestamp>
	re := regexp.MustCompile(`/archives/([A-Z0-9]+)/p([0-9]+)`)
	matches := re.FindStringSubmatch(rawURL)
	if len(matches) >= 3 {
		channelID = matches[1]
		// Convert timestamp from p<timestamp> to <timestamp>.<ms>
		ts := matches[2]
		if len(ts) >= 10 {
			// Insert dot: seconds.milliseconds
			timestamp = ts[:10] + "." + ts[10:]
		}
	}
	return
}

// formatMessages formats message list for output
func formatMessages(messages []Message) string {
	if len(messages) == 0 {
		return "No messages found."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d message(s):\n\n", len(messages)))
	for _, msg := range messages {
		tsb := msg.Timestamp
		if strings.Contains(tsb, ".") {
			parts := strings.Split(tsb, ".")
			ms := parts[1]
			if len(ms) > 3 {
				ms = ms[:3]
			}
			tsb = parts[0] + "." + ms
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", tsb, msg.User, msg.Text))
		if msg.ReplyCount > 0 {
			sb.WriteString(fmt.Sprintf("  (thread: %d replies)\n", msg.ReplyCount))
		}
	}
	return sb.String()
}

// formatChannels formats channel list for output
func formatChannels(channels []Conversation) string {
	if len(channels) == 0 {
		return "No channels found."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d channel(s):\n\n", len(channels)))
	for _, ch := range channels {
		name := ch.Name
		if name == "" {
			name = "(DM)"
		}
		typeStr := ""
		switch {
		case ch.IsIM:
			typeStr = "DM"
		case ch.IsPrivate:
			typeStr = "private"
		default:
			typeStr = "public"
		}
		sb.WriteString(fmt.Sprintf("- %s (%s) [%s]\n", name, ch.ID, typeStr))
	}
	return sb.String()
}

// formatSearchResults formats search results for output
func formatSearchResults(results *SearchResult) string {
	if len(results.Matches) == 0 {
		return "No messages found matching your query."
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d match(es):\n\n", results.Pagination.TotalCount))
	for i, match := range results.Matches {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s: %s\n", i+1, match.Channel.Name, match.User, match.Text))
		sb.WriteString(fmt.Sprintf("   %s\n\n", match.Permalink))
	}
	if results.Pagination.TotalPages > 1 {
		sb.WriteString(fmt.Sprintf("... and %d more results\n", results.Pagination.TotalCount-results.Pagination.PerPage))
	}
	return sb.String()
}