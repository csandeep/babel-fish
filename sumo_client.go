// sumo_client.go — HTTP client for SumoLogic Search Jobs API
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// SumoClient handles authenticated requests to SumoLogic's Search Jobs API.
type SumoClient struct {
	httpClient *http.Client
	accessID   string
	accessKey  string
	baseURL    string
}

// SearchJobRequest is the payload for creating a search job.
type SearchJobRequest struct {
	Query              string `json:"query"`
	From               string `json:"from"`
	To                 string `json:"to"`
	TimeZone           string `json:"timeZone,omitempty"`
	ByReceiptTime      bool   `json:"byReceiptTime,omitempty"`
	RequireRawMessages bool   `json:"requireRawMessages,omitempty"`
}

// SearchJobResponse is returned when a search job is created.
type SearchJobResponse struct {
	ID   string `json:"id"`
	Link struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"link"`
}

// SearchJobStatus represents the current state of a search job.
type SearchJobStatus struct {
	State        string `json:"state"`
	MessageCount int    `json:"messageCount"`
	RecordCount  int    `json:"recordCount"`
}

// SearchJobMessages holds the raw log messages from a completed search job.
type SearchJobMessages struct {
	Fields   []SumoMessageField `json:"fields"`
	Messages []SumoMessage      `json:"messages"`
}

// SearchJobRecords holds the aggregated records from a completed search job.
type SearchJobRecords struct {
	Fields  []SumoMessageField `json:"fields"`
	Records []SumoMessage      `json:"records"`
}

// SumoMessageField describes a field in the message schema.
type SumoMessageField struct {
	Name      string `json:"name"`
	FieldType string `json:"fieldType"`
	KeyField  bool   `json:"keyField"`
}

// SumoMessage wraps a single log message as a map of field names to values.
type SumoMessage struct {
	Map map[string]string `json:"map"`
}

// NewSumoClient creates a client configured with Access ID / Access Key Basic Auth.
// The Search Jobs API requires cookie state to be preserved across requests so a
// cookie jar is attached to the underlying HTTP client.
func NewSumoClient(accessID, accessKey, baseURL string) (*SumoClient, error) {
	if accessID == "" {
		return nil, fmt.Errorf("sumo access ID is required")
	}
	if accessKey == "" {
		return nil, fmt.Errorf("sumo access key is required")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	return &SumoClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		accessID:  accessID,
		accessKey: accessKey,
		baseURL:   baseURL,
	}, nil
}

// SearchLogsResult is a unified result that may contain raw messages, aggregated
// records, or both depending on the query type.
type SearchLogsResult struct {
	Messages *SearchJobMessages
	Records  *SearchJobRecords
}

// SearchLogs creates a search job, polls until completion, fetches messages
// or records depending on query type, and deletes the job.
// The isAggregation flag must be true when the query contains aggregation operators
// (count, sum, avg, max, min, group by, etc.) — these produce records, not messages.
func (c *SumoClient) SearchLogs(query, from, to, timeZone string, limit int, isAggregation bool) (*SearchLogsResult, error) {
	jobID, err := c.createSearchJob(query, from, to, timeZone, isAggregation)
	if err != nil {
		return nil, err
	}
	defer c.deleteSearchJob(jobID)

	deadline := time.Now().Add(5 * time.Minute)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("search job timed out after 5 minutes")
		}

		status, err := c.getSearchJobStatus(jobID)
		if err != nil {
			return nil, err
		}
		log.Printf("[babel-fish] Search job %s status: %s (messages: %d, records: %d)", jobID, status.State, status.MessageCount, status.RecordCount)

		switch status.State {
		case "DONE GATHERING RESULTS":
			return c.fetchResults(jobID, limit, isAggregation)
		case "CANCELLED":
			return nil, fmt.Errorf("search job was cancelled")
		case "NOT STARTED", "GATHERING RESULTS":
			time.Sleep(5 * time.Second)
		default:
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *SumoClient) fetchResults(jobID string, limit int, isAggregation bool) (*SearchLogsResult, error) {
	result := &SearchLogsResult{}

	if isAggregation {
		// Aggregation queries (e.g. "| count by foo") only produce records.
		// The /messages endpoint returns 400 when requireRawMessages is false.
		records, err := c.getSearchJobRecords(jobID, limit)
		if err != nil {
			return nil, err
		}
		result.Records = records
		return result, nil
	}

	// Raw log queries produce messages.
	messages, err := c.getSearchJobMessages(jobID, limit)
	if err != nil {
		return nil, err
	}
	result.Messages = messages
	return result, nil
}

func (c *SumoClient) basicAuth() string {
	creds := c.accessID + ":" + c.accessKey
	return base64.StdEncoding.EncodeToString([]byte(creds))
}

func (c *SumoClient) createSearchJob(query, from, to, timeZone string, isAggregation bool) (string, error) {
	reqBody := SearchJobRequest{
		Query:              query,
		From:               from,
		To:                 to,
		TimeZone:           timeZone,
		RequireRawMessages: !isAggregation,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/v1/search/jobs", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+c.basicAuth())

	log.Printf("[babel-fish] Creating SumoLogic search job: %s", query)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchJobResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[babel-fish] Created search job: %s", result.ID)
	return result.ID, nil
}

func (c *SumoClient) getSearchJobStatus(jobID string) (*SearchJobStatus, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v1/search/jobs/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+c.basicAuth())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchJobStatus
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (c *SumoClient) getSearchJobMessages(jobID string, limit int) (*SearchJobMessages, error) {
	u := c.baseURL + "/v1/search/jobs/" + jobID + "/messages?offset=0"
	if limit > 0 {
		u += fmt.Sprintf("&limit=%d", limit)
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+c.basicAuth())

	log.Printf("[babel-fish] Fetching messages for job %s", jobID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchJobMessages
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[babel-fish] Fetched %d messages", len(result.Messages))
	return &result, nil
}

func (c *SumoClient) getSearchJobRecords(jobID string, limit int) (*SearchJobRecords, error) {
	u := c.baseURL + "/v1/search/jobs/" + jobID + "/records?offset=0"
	if limit > 0 {
		u += fmt.Sprintf("&limit=%d", limit)
	}

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Basic "+c.basicAuth())

	log.Printf("[babel-fish] Fetching records for job %s", jobID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result SearchJobRecords
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[babel-fish] Fetched %d records", len(result.Records))
	return &result, nil
}

func (c *SumoClient) deleteSearchJob(jobID string) {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/v1/search/jobs/"+jobID, nil)
	if err != nil {
		log.Printf("[babel-fish] Failed to build delete request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Basic "+c.basicAuth())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[babel-fish] Failed to delete search job: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[babel-fish] Deleted search job %s (status %d)", jobID, resp.StatusCode)
}
