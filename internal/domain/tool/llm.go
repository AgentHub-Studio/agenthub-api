package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// llmConfig holds the tenant-configured LLM provider settings.
// Stored under settings key "llm.tool.config".
type llmConfig struct {
	Provider string `json:"provider"` // "openai" | "anthropic" | "openrouter" | "ollama"
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseUrl,omitempty"` // override for ollama / openrouter
	Model    string `json:"model"`
}

var llmHTTPClient = &http.Client{Timeout: 60 * time.Second}

// callLLM sends a single-turn request to an OpenAI-compatible chat completions endpoint
// and returns the assistant message content.
func callLLM(ctx context.Context, cfg llmConfig, systemPrompt, userPrompt string) (string, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		switch cfg.Provider {
		case "anthropic":
			return callAnthropic(ctx, cfg, systemPrompt, userPrompt)
		case "openrouter":
			baseURL = "https://openrouter.ai/api/v1"
		case "ollama":
			baseURL = "http://localhost:11434/v1"
		default: // openai
			baseURL = "https://api.openai.com/v1"
		}
	}

	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Model    string    `json:"model"`
		Messages []message `json:"messages"`
	}

	messages := []message{}
	if systemPrompt != "" {
		messages = append(messages, message{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, message{Role: "user", Content: userPrompt})

	payload := request{Model: cfg.Model, Messages: messages}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("llm: empty response")
	}
	return result.Choices[0].Message.Content, nil
}

// callAnthropic calls the Anthropic Messages API.
func callAnthropic(ctx context.Context, cfg llmConfig, systemPrompt, userPrompt string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Model     string    `json:"model"`
		MaxTokens int       `json:"max_tokens"`
		System    string    `json:"system,omitempty"`
		Messages  []message `json:"messages"`
	}

	payload := request{
		Model:     cfg.Model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages:  []message{{Role: "user", Content: userPrompt}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("anthropic: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("anthropic: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := llmHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("anthropic: decode: %w", err)
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("anthropic: no text content in response")
}
