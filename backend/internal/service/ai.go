// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// AI assistant client: OpenAI-compatible protocol (works with OpenAI / Ollama / vLLM / LM Studio, etc.)

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AISettings struct {
	Enabled bool
	BaseURL string // e.g. http://127.0.0.1:11434/v1 (ollama), https://api.openai.com/v1
	APIKey  string // can be left empty for local ollama
	Model   string // e.g. qwen2.5:7b / gpt-4o-mini
	Timeout time.Duration
}

// LoadAISettings reads AI settings from system config
func LoadAISettings() AISettings {
	m := SystemConfigMap()
	timeout := 120 * time.Second
	if n, e := strconv.Atoi(m["ai_timeout_sec"]); e == nil && n >= 5 && n <= 600 {
		timeout = time.Duration(n) * time.Second
	}
	return AISettings{
		Enabled: m["ai_enabled"] == "true",
		BaseURL: strings.TrimRight(strings.TrimSpace(m["ai_base_url"]), "/"),
		APIKey:  m["ai_api_key"],
		Model:   strings.TrimSpace(m["ai_model"]),
		Timeout: timeout,
	}
}

type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIChat calls the OpenAI-compatible /chat/completions endpoint and returns the reply text
func AIChat(s AISettings, systemPrompt, userPrompt string) (string, error) {
	if s.BaseURL == "" || s.Model == "" {
		return "", fmt.Errorf("AI is not configured: set the base URL and model in System Settings first")
	}
	payload := map[string]any{
		"model": s.Model,
		"messages": []aiMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		"stream": false,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", s.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	}
	client := &http.Client{Timeout: s.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI service connection failed: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("failed to parse AI response (HTTP %d)", resp.StatusCode)
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", fmt.Errorf("AI service error: %s", out.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("AI service returned HTTP %d", resp.StatusCode)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI service returned an empty reply")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
