// sumo_tools.go — MCP tool definitions for SumoLogic
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SumoToolHandler holds the SumoLogic client and implements MCP tools.
type SumoToolHandler struct {
	client *SumoClient
}

// NewSumoToolHandler creates a new tool handler for SumoLogic.
func NewSumoToolHandler(client *SumoClient) *SumoToolHandler {
	return &SumoToolHandler{client: client}
}

// Input types for each tool

type sumoSearchLogsInput struct {
	Query     string `json:"query"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	TimeZone  string `json:"time_zone,omitempty"`
	TimeRange string `json:"time_range,omitempty"`
	Limit     int    `json:"limit"`
}

type sumoSearchTracesInput struct {
	ServiceName string `json:"service_name,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	TimeRange   string `json:"time_range,omitempty"`
	Limit       int    `json:"limit"`
}

// isAggregationQuery detects if a SumoLogic query uses aggregation operators
// that produce records instead of raw messages.
func isAggregationQuery(query string) bool {
	lower := strings.ToLower(query)
	ops := []string{
		"| count", "count by", "count(",
		"| sum", "sum(",
		"| avg", "avg(",
		"| min", "min(",
		"| max", "max(",
		"| group by", "groupby",
		"| aggregate", "aggregate(",
		"| transpose", "transpose(",
		"| join", "join(",
		"| timeslice", "timeslice(",
		"| outlier", "outlier(",
		"| formatDate", "formatDate(",
		"| parseDate", "parseDate(",
		"| first", "first(",
		"| last", "last(",
		"| most_recent", "most_recent(",
		"| least_recent", "least_recent(",
		"| compare", "compare(",
		"| subquery", "subquery(",
	}
	for _, op := range ops {
		if strings.Contains(lower, op) {
			return true
		}
	}
	return false
}

// Output type (all tools return text)
type sumoTextOutput struct {
	Text string `json:"text"`
}

// configureSumoTools adds all SumoLogic tools to the MCP server.
func configureSumoTools(server *mcp.Server, handler *SumoToolHandler) {
	// Tool: sumo_search_logs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sumo_search_logs",
		Description: "Search logs in SumoLogic using a query. Supports relative time ranges like '15m', '1h', '1d' or absolute ISO timestamps.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input sumoSearchLogsInput) (*mcp.CallToolResult, sumoTextOutput, error) {
		from, to, err := resolveTimeRange(input.From, input.To, input.TimeRange)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, sumoTextOutput{}, nil
		}

		limit := input.Limit
		if limit <= 0 {
			limit = 100
		}
		if limit > 10000 {
			limit = 10000
		}

		isAgg := isAggregationQuery(input.Query)
		results, err := handler.client.SearchLogs(input.Query, from, to, input.TimeZone, limit, isAgg)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error searching logs: %v", err)}},
				IsError: true,
			}, sumoTextOutput{}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatSumoResults(results)}},
		}, sumoTextOutput{}, nil
	})

	// Tool: sumo_search_error_traces
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sumo_search_error_traces",
		Description: "Search for error traces and exceptions in SumoLogic logs. Filters for error/exception/crash by default.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input sumoSearchTracesInput) (*mcp.CallToolResult, sumoTextOutput, error) {
		from, to, err := resolveTimeRange(input.From, input.To, input.TimeRange)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)}},
				IsError: true,
			}, sumoTextOutput{}, nil
		}

		limit := input.Limit
		if limit <= 0 {
			limit = 100
		}
		if limit > 10000 {
			limit = 10000
		}

		query := buildErrorTraceQuery(input.ServiceName, input.TraceID)
		isAgg := isAggregationQuery(query)
		results, err := handler.client.SearchLogs(query, from, to, "", limit, isAgg)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Error searching traces: %v", err)}},
				IsError: true,
			}, sumoTextOutput{}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: formatSumoResults(results)}},
		}, sumoTextOutput{}, nil
	})
}

// resolveTimeRange translates relative ranges or explicit from/to into ISO timestamps.
func resolveTimeRange(from, to, relRange string) (string, string, error) {
	now := time.Now().UTC()

	if from == "" && to == "" && relRange == "" {
		return "", "", fmt.Errorf("either provide 'from'/'to' timestamps or a 'time_range' like '15m', '1h', '1d'")
	}

	if relRange != "" {
		d, err := parseDuration(relRange)
		if err != nil {
			return "", "", err
		}
		fromTime := now.Add(-d)
		return fromTime.Format(time.RFC3339), now.Format(time.RFC3339), nil
	}

	if from == "" || to == "" {
		return "", "", fmt.Errorf("both 'from' and 'to' must be provided when 'time_range' is not set")
	}

	return from, to, nil
}

// parseDuration converts a simple relative duration string to a time.Duration.
// Supported suffixes: m (minutes), h (hours), d (days), w (weeks).
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1]
	valStr := s[:len(s)-1]

	var mult time.Duration
	switch unit {
	case 'm':
		mult = time.Minute
	case 'h':
		mult = time.Hour
	case 'd':
		mult = 24 * time.Hour
	case 'w':
		mult = 7 * 24 * time.Hour
	default:
		return time.ParseDuration(s)
	}

	var val int
	if _, err := fmt.Sscanf(valStr, "%d", &val); err != nil {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	return time.Duration(val) * mult, nil
}

// buildErrorTraceQuery constructs a SumoLogic query that targets errors/exceptions.
func buildErrorTraceQuery(serviceName, traceID string) string {
	var parts []string
	if serviceName != "" {
		parts = append(parts, fmt.Sprintf("_sourceCategory=%s", serviceName))
	}
	if traceID != "" {
		parts = append(parts, fmt.Sprintf("\"%s\"", traceID))
	}
	parts = append(parts, "\"error\" OR \"exception\" OR \"stacktrace\" OR \"stack trace\" OR \"ERROR\" OR \"EXCEPTION\" OR \"FATAL\" OR \"CRASH\"")
	return strings.Join(parts, " AND ")
}

// formatSumoResults formats SumoLogic messages and/or records into human-readable text.
func formatSumoResults(result *SearchLogsResult) string {
	var sb strings.Builder

	if result.Messages != nil && len(result.Messages.Messages) > 0 {
		var fields []string
		for _, f := range result.Messages.Fields {
			fields = append(fields, f.Name)
		}
		sb.WriteString(fmt.Sprintf("=== Messages: %d ===\n\n", len(result.Messages.Messages)))
		for i, msg := range result.Messages.Messages {
			sb.WriteString(fmt.Sprintf("--- Message %d ---\n", i+1))
			for _, field := range fields {
				if val, ok := msg.Map[field]; ok && val != "" {
					sb.WriteString(fmt.Sprintf("%s: %s\n", field, val))
				}
			}
			sb.WriteString("\n")
		}
	}

	if result.Records != nil && len(result.Records.Records) > 0 {
		var fields []string
		for _, f := range result.Records.Fields {
			fields = append(fields, f.Name)
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("=== Aggregated Records: %d ===\n\n", len(result.Records.Records)))
		for i, rec := range result.Records.Records {
			sb.WriteString(fmt.Sprintf("--- Record %d ---\n", i+1))
			for _, field := range fields {
				if val, ok := rec.Map[field]; ok && val != "" {
					sb.WriteString(fmt.Sprintf("%s: %s\n", field, val))
				}
			}
			sb.WriteString("\n")
		}
	}

	if sb.Len() == 0 {
		return "No messages or records found."
	}
	return sb.String()
}

// formatSumoResultsJSON returns the result as compact JSON text.
func formatSumoResultsJSON(result *SearchLogsResult) string {
	var data any
	if result.Records != nil && len(result.Records.Records) > 0 {
		data = result.Records.Records
	} else if result.Messages != nil {
		data = result.Messages.Messages
	} else {
		return "No messages or records found."
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling results: %v", err)
	}
	return string(out)
}
