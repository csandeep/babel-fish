// main.go - Entry point, MCP server setup
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var (
	slackTokenFlag    string
	slackCookieFlag   string
	slackCookieDSFlag string

	sumoAccessIDFlag  string
	sumoAccessKeyFlag string
	sumoBaseURLFlag   string
)

func initFlags() {
	flag.StringVar(&slackTokenFlag, "slack-token", "", "Slack xoxc- session token")
	flag.StringVar(&slackCookieFlag, "slack-cookie", "", "Slack xoxd- session cookie (value of cookie 'd')")
	flag.StringVar(&slackCookieDSFlag, "slack-cookie-d-s", "", "Optional Slack SSO d-s cookie")

	flag.StringVar(&sumoAccessIDFlag, "sumo-access-id", "", "SumoLogic Access ID")
	flag.StringVar(&sumoAccessKeyFlag, "sumo-access-key", "", "SumoLogic Access Key")
	flag.StringVar(&sumoBaseURLFlag, "sumo-base-url", "https://api.sumologic.com/api", "SumoLogic API base URL")

	flag.Parse()

	// Fall back to environment variables for backward compatibility
	if slackTokenFlag == "" {
		slackTokenFlag = os.Getenv("SLACK_TOKEN")
	}
	if slackCookieFlag == "" {
		slackCookieFlag = os.Getenv("SLACK_COOKIE")
	}
	if slackCookieDSFlag == "" {
		slackCookieDSFlag = os.Getenv("SLACK_COOKIE_D_S")
	}
	if sumoAccessIDFlag == "" {
		sumoAccessIDFlag = os.Getenv("SUMO_ACCESS_ID")
	}
	if sumoAccessKeyFlag == "" {
		sumoAccessKeyFlag = os.Getenv("SUMO_ACCESS_KEY")
	}
	if sumoBaseURLFlag == "" {
		sumoBaseURLFlag = os.Getenv("SUMO_BASE_URL")
	}
}

func main() {
	initFlags()

	// Validate that at least one service is configured
	hasSlack := slackTokenFlag != "" && slackCookieFlag != ""
	hasSumo := sumoAccessIDFlag != "" && sumoAccessKeyFlag != ""

	if !hasSlack && !hasSumo {
		log.Println("Error: At least one service must be configured (Slack or SumoLogic).")
		log.Println("")
		log.Println("Slack options (required to enable Slack tools):")
		log.Println("  --slack-token      xoxc-... session token")
		log.Println("  --slack-cookie     xoxd-... session cookie value")
		log.Println("  --slack-cookie-d-s Optional SSO cookie")
		log.Println("")
		log.Println("SumoLogic options (required to enable SumoLogic tools):")
		log.Println("  --sumo-access-id   SumoLogic Access ID")
		log.Println("  --sumo-access-key  SumoLogic Access Key")
		log.Println("  --sumo-base-url    SumoLogic API base URL (default: https://api.sumologic.com/api)")
		log.Println("")
		log.Println("Environment variables are also accepted for backward compatibility.")
		os.Exit(1)
	}

	// Create MCP server
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "babel-fish",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{
			Instructions: buildInstructions(),
		},
	)

	// Configure Slack tools and resources if credentials provided
	if hasSlack {
		client, err := NewSlackClient(slackTokenFlag, slackCookieFlag, slackCookieDSFlag)
		if err != nil {
			log.Fatalf("Failed to create Slack client: %v", err)
		}
		handler := NewToolHandler(client)
		configureTools(server, handler)
		configureResources(server, client)
		log.Println("[babel-fish] Slack tools enabled")
	}

	// Configure SumoLogic tools if credentials provided
	if hasSumo {
		if sumoBaseURLFlag == "" {
			sumoBaseURLFlag = "https://api.sumologic.com/api"
		}
		sumoClient, err := NewSumoClient(sumoAccessIDFlag, sumoAccessKeyFlag, sumoBaseURLFlag)
		if err != nil {
			log.Fatalf("Failed to create SumoLogic client: %v", err)
		}
		sumoHandler := NewSumoToolHandler(sumoClient)
		configureSumoTools(server, sumoHandler)
		log.Println("[babel-fish] SumoLogic tools enabled")
	}

	log.Println("[babel-fish] Starting MCP server on stdio")

	// Run using stdio transport
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func buildInstructions() string {
	var parts []string
	parts = append(parts, "Babel-fish MCP Server\n")
	parts = append(parts, "This server provides access to Slack and/or SumoLogic depending on the credentials supplied at startup.\n")

	if slackTokenFlag != "" && slackCookieFlag != "" {
		parts = append(parts, "Slack tools (browser session credentials):\n")
		parts = append(parts, "- slack_list_channels: List accessible channels\n")
		parts = append(parts, "- slack_read_messages: Read messages from a channel\n")
		parts = append(parts, "- slack_read_thread: Read thread replies\n")
		parts = append(parts, "- slack_search_messages: Search across workspace\n")
		parts = append(parts, "- slack_get_permalink: Parse a Slack permalink\n")
		parts = append(parts, "Resources:\n")
		parts = append(parts, "- slack://channels - List of all accessible channels\n")
		parts = append(parts, "- slack://channel/{id}/messages - Messages from a specific channel\n")
		parts = append(parts, "Note: Session cookies may expire. If authentication fails, re-extract credentials from your browser.\n")
	}

	if sumoAccessIDFlag != "" && sumoAccessKeyFlag != "" {
		parts = append(parts, "SumoLogic tools:\n")
		parts = append(parts, "- sumo_search_logs: Search logs with a query and time range\n")
		parts = append(parts, "- sumo_search_error_traces: Search for errors and stack traces\n")
	}

	return strings.Join(parts, "\n")
}

