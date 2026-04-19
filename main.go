// main.go - Entry point, MCP server setup
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	// Check for required environment variables
	if os.Getenv("SLACK_TOKEN") == "" || os.Getenv("SLACK_COOKIE") == "" {

		log.Println("Error: SLACK_TOKEN and SLACK_COOKIE environment variables are required")
		log.Println("")
		log.Println("To extract session credentials from Slack:")
		log.Println("1. Open Slack in your browser (app.slack.com)")
		log.Println("2. Open DevTools (F12) → Application → Storage → localStorage")
		log.Println("3. Look for key starting with 'xoxc-' - this is your SLACK_TOKEN")
		log.Println("4. Go to Application → Cookies → https://app.slack.com")
		log.Println("5. Find cookie 'd' starting with 'xoxd-' - this is your SLACK_COOKIE")
		log.Println("")
		log.Println("Then run with:")
		log.Println("  SLACK_TOKEN=xoxc-... SLACK_COOKIE=xoxd-... ./babel-fish")
		os.Exit(1)
	}

	// Create Slack client
	client, err := NewSlackClient()
	if err != nil {
		log.Fatalf("Failed to create Slack client: %v", err)
	}

	// Create MCP server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "babel-fish",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{

			Instructions: "Babel-fish Slack Session MCP Server\n\n" +
				"This server provides access to Slack using browser session credentials.\n" +
				"Available tools:\n" +
				"- slack_list_channels: List accessible channels\n" +
				"- slack_read_messages: Read messages from a channel\n" +
				"- slack_read_thread: Read thread replies\n" +
				"- slack_search_messages: Search across workspace\n" +
				"- slack_get_permalink: Parse a Slack permalink\n\n" +
				"Available resources:\n" +
				"- slack://channels - List of all accessible channels\n" +
				"- slack://channel/{id}/messages - Messages from a specific channel\n\n" +
				"Note: Session cookies may expire. If authentication fails, re-extract credentials from your browser.",
		},
	)

	// Configure tools
	handler := NewToolHandler(client)
	configureTools(server, handler)

	// Configure resources
	configureResources(server, client)

	log.Println("[babel-fish] Starting MCP server on stdio")
	log.Println("[babel-fish] SLACK_TOKEN present:", os.Getenv("SLACK_TOKEN") != "")
	log.Println("[babel-fish] SLACK_COOKIE present:", os.Getenv("SLACK_COOKIE") != "")
	log.Println("[babel-fish] SLACK_COOKIE_D_S present:", os.Getenv("SLACK_COOKIE_D_S") != "")

	// Run using stdio transport
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
