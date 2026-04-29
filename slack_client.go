// slack_client.go - Authenticated HTTP client for Slack Web API
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SlackClient handles authenticated requests to Slack's Web API using session credentials
type SlackClient struct {
	httpClient *http.Client
	token      string
	cookie     string
	cookieDS   string // For SSO workspaces
	baseURL    string
}

// NewSlackClient creates a new client from the provided session credentials.
func NewSlackClient(token, cookie, cookieDS string) (*SlackClient, error) {
	if token == "" {
		return nil, fmt.Errorf("slack token is required")
	}

	if cookie == "" {
		return nil, fmt.Errorf("slack cookie is required")
	}

	return &SlackClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
		cookie:     cookie,
		cookieDS:   cookieDS,
		baseURL:    "https://slack.com/api/",
	}, nil
}

// buildCookieString constructs the Cookie header value
func (c *SlackClient) buildCookieString() string {
	cookies := "d=" + c.cookie
	if c.cookieDS != "" {
		cookies += "; d-s=" + c.cookieDS
	}
	return cookies
}

// post makes a POST request to the Slack API
func (c *SlackClient) post(endpoint string, params url.Values, result interface{}) error {
	log.Printf("[babel-fish] POST %s params=%s", endpoint, params.Encode())
	req, err := http.NewRequest(http.MethodPost, c.baseURL+endpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Cookie", c.buildCookieString())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Execute with retry
	var resp *http.Response
	for attempts := 0; attempts < 3; attempts++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode != http.StatusTooManyRequests {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempts < 2 {
			log.Printf("[babel-fish] Retrying %s (attempt %d): %v", endpoint, attempts+2, err)
			time.Sleep(time.Duration(attempts+1) * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[babel-fish] POST %s → HTTP %d", endpoint, resp.StatusCode)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &slackResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !slackResp.OK {
		log.Printf("[babel-fish] POST %s → Slack error: %s", endpoint, slackResp.Error)
		return fmt.Errorf("slack API error: %s", slackResp.Error)
	}

	if result != nil {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	log.Printf("[babel-fish] POST %s → ok", endpoint)
	return nil
}

// ConversationsHistoryResponse represents the response from conversations.history
type ConversationsHistoryResponse struct {
	OK       bool      `json:"ok"`
	Messages []Message `json:"messages"`
	HasMore  bool      `json:"has_more"`
}

// GetConversationHistory fetches messages from a conversation
func (c *SlackClient) GetConversationHistory(channelID string, limit int, oldest, latest string) (*ConversationsHistoryResponse, error) {
	params := url.Values{}
	params.Set("channel", channelID)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if oldest != "" {
		params.Set("oldest", oldest)
	}
	if latest != "" {
		params.Set("latest", latest)
	}

	var result ConversationsHistoryResponse
	if err := c.post("conversations.history", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ConversationsRepliesResponse represents the response from conversations.replies
type ConversationsRepliesResponse struct {
	OK       bool      `json:"ok"`
	Messages []Message `json:"messages"`
	HasMore  bool      `json:"has_more"`
}

// GetConversationReplies fetches replies in a thread
func (c *SlackClient) GetConversationReplies(channelID, threadTS string, limit int) (*ConversationsRepliesResponse, error) {
	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("ts", threadTS)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	var result ConversationsRepliesResponse
	if err := c.post("conversations.replies", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ConversationsListResponse represents the response from conversations.list
type ConversationsListResponse struct {
	OK               bool           `json:"ok"`
	Channels         []Conversation `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor,omitempty"`
	} `json:"response_metadata"`
}

// ListConversations fetches accessible channels
func (c *SlackClient) ListConversations(types string, limit int) ([]Conversation, error) {
	params := url.Values{}
	if types == "" {
		types = "public_channel,private_channel,mpim,im"
	}
	params.Set("types", types)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	params.Set("exclude_archived", "false")

	var result ConversationsListResponse
	if err := c.post("conversations.list", params, &result); err != nil {
		return nil, err
	}
	return result.Channels, nil
}

// SearchMessagesResponse represents the response from search.messages
type SearchMessagesResponse struct {
	OK      bool         `json:"ok"`
	Matches SearchResult `json:"messages"`
}

// SearchMessages searches for messages across the workspace
func (c *SlackClient) SearchMessages(query string, count int) (*SearchResult, error) {
	params := url.Values{}
	params.Set("query", query)
	if count > 0 {
		params.Set("count", fmt.Sprintf("%d", count))
	}

	var result SearchMessagesResponse
	if err := c.post("search.messages", params, &result); err != nil {
		return nil, err
	}
	return &result.Matches, nil
}

// Message represents a Slack message
type Message struct {
	Type       string `json:"type"`
	User       string `json:"user"`
	Text       string `json:"text"`
	Timestamp  string `json:"ts"`
	ThreadTS   string `json:"thread_ts,omitempty"`
	ReplyCount int    `json:"reply_count,omitempty"`
}

// Conversation represents a Slack channel/conversation
type Conversation struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsChannel  bool   `json:"is_channel"`
	IsGroup    bool   `json:"is_group"`
	IsIM       bool   `json:"is_im"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	NumMembers int    `json:"num_members"`
	Created    int64  `json:"created"`
	Creator    string `json:"creator"`
}

// SearchResult represents a search result message
type SearchResult struct {
	Matches []struct {
		Type    string `json:"type"`
		Channel struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"channel"`
		User      string `json:"user"`
		Username  string `json:"username"`
		Text      string `json:"text"`
		Timestamp string `json:"ts"`
		Permalink string `json:"permalink"`
	} `json:"matches"`
	Pagination struct {
		Page       int `json:"page"`
		TotalPages int `json:"total_pages"`
		PerPage    int `json:"per_page"`
		TotalCount int `json:"total_count"`
	} `json:"pagination"`
}
