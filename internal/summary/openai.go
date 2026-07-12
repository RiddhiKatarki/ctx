package summary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RiddhiKatarki/ctx/pkg/types"
)

// OpenAIProvider generates summaries by calling an OpenAI-compatible
// chat completions endpoint. The base URL is configurable so it works
// with OpenAI, Venice, Ollama, vLLM, or any compatible provider.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (p *OpenAIProvider) Summarize(snapshot types.Snapshot) (*types.Summary, error) {
	prompt := buildLLMPrompt(snapshot)

	req := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt()},
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(p.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("LLM error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := chatResp.Choices[0].Message.Content
	return ParseMarkdownSummary(content), nil
}

func systemPrompt() string {
	return `You are a software development context summarizer. Generate a structured project summary using the following sections, each preceded by a "## " markdown heading:

## Current Objective
## Completed Work
## Remaining Tasks
## Known Bugs
## Architecture Decisions
## Files To Read First
## Previous Failed Approaches
## Suggested Next Prompt
## Estimated Reading Time

Be concise and specific. Base your summary on the provided snapshot data. Do not invent information that isn't present in the snapshot.`
}

func buildLLMPrompt(snapshot types.Snapshot) string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Project: %s\n", snapshot.Metadata.ProjectName)
	fmt.Fprintf(&buf, "Branch: %s\n", snapshot.Metadata.Branch)
	fmt.Fprintf(&buf, "HEAD: %s\n", snapshot.Git.HeadCommit)
	fmt.Fprintf(&buf, "Dirty: %t\n", snapshot.Git.Dirty)
	if snapshot.Git.RemoteURL != "" {
		fmt.Fprintf(&buf, "Remote: %s\n", snapshot.Git.RemoteURL)
	}

	buf.WriteString("\nModified Files:\n")
	for _, f := range snapshot.Files {
		fmt.Fprintf(&buf, "  - %s\n", f)
	}

	if len(snapshot.Prompts) > 0 {
		buf.WriteString("\nPrompt History:\n")
		for _, p := range snapshot.Prompts {
			fmt.Fprintf(&buf, "  [%s]: %s\n", p.Role, p.Content)
		}
	}

	if snapshot.Diff != "" {
		diffPreview := snapshot.Diff
		if len(diffPreview) > 4000 {
			diffPreview = diffPreview[:4000] + "\n... (truncated)"
		}
		fmt.Fprintf(&buf, "\nGit Diff:\n%s\n", diffPreview)
	}

	buf.WriteString("\nGenerate a structured summary with all 9 sections listed in the system prompt.\n")
	return buf.String()
}
