package robot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"driftbottle/internal/crypto"
	"driftbottle/internal/sysconfig"
)

// LLMClient 调用 OpenAI 兼容 API，semaphore 控制并发。
type LLMClient struct {
	http    *http.Client
	mu      sync.Mutex
	sem     chan struct{}
	semSize int
}

func newLLMClient(defaultTenant int64) *LLMClient {
	concurrency := sysconfig.GetInt(defaultTenant, sysconfig.KeyLLMConcurrency)
	if concurrency <= 0 {
		concurrency = 10
	}
	return &LLMClient{
		http:    &http.Client{Timeout: 30 * time.Second},
		sem:     make(chan struct{}, concurrency),
		semSize: concurrency,
	}
}

// RebuildSem 当并发数 sysconfig 改变时重建 semaphore（可选，当前启动时读一次即可）。
func (c *LLMClient) RebuildSem(defaultTenant int64) {
	n := sysconfig.GetInt(defaultTenant, sysconfig.KeyLLMConcurrency)
	if n <= 0 {
		n = 10
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if n == c.semSize {
		return
	}
	c.sem = make(chan struct{}, n)
	c.semSize = n
}

type llmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmRequest struct {
	Model       string       `json:"model"`
	Messages    []llmMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature"`
}

type llmResponse struct {
	Choices []struct {
		Message llmMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用 LLM 并返回回复文本；history 为结构化历史消息（user/assistant 交替），失败返回非空 error。
func (c *LLMClient) Chat(ctx context.Context, tenantID int64, systemPrompt string, history []llmMessage, userText string) (string, error) {
	endpoint := sysconfig.GetString(tenantID, sysconfig.KeyLLMAPIEndpoint)
	if endpoint == "" {
		return "", errors.New("llm_api_endpoint 未配置")
	}
	apiKeyEnc := sysconfig.GetString(tenantID, sysconfig.KeyLLMAPIKey)
	apiKey := apiKeyEnc
	if crypto.Enabled() && apiKeyEnc != "" {
		var err error
		apiKey, err = crypto.Decrypt(apiKeyEnc)
		if err != nil {
			log.Printf("[robot/llm] api key 解密失败: %v", err)
			apiKey = apiKeyEnc // 回退：视为明文（开发环境）
		}
	}

	model := sysconfig.GetString(tenantID, sysconfig.KeyLLMModel)
	if model == "" {
		model = "gpt-4o-mini"
	}
	maxTokens := sysconfig.GetInt(tenantID, sysconfig.KeyLLMMaxTokens)
	if maxTokens <= 0 {
		maxTokens = 200
	}
	temp := float64(sysconfig.GetInt(tenantID, sysconfig.KeyLLMTemperature)) / 10.0

	// 正确结构：system → 历史 user/assistant 交替 → 当前 user
	messages := make([]llmMessage, 0, 2+len(history))
	messages = append(messages, llmMessage{Role: "system", Content: systemPrompt})
	messages = append(messages, history...)
	messages = append(messages, llmMessage{Role: "user", Content: userText})

	req := llmRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temp,
	}
	body, _ := json.Marshal(req)

	// 占用 semaphore
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var llmResp llmResponse
	if e := json.Unmarshal(raw, &llmResp); e != nil {
		return "", fmt.Errorf("llm json: %w, body=%s", e, string(raw[:min(len(raw), 200)]))
	}
	if llmResp.Error != nil {
		return "", errors.New(llmResp.Error.Message)
	}
	if len(llmResp.Choices) == 0 {
		return "", errors.New("llm 返回空 choices")
	}
	return llmResp.Choices[0].Message.Content, nil
}
