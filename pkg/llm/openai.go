package llm

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/logger"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

type OpenAIClient struct {
	client         *openai.Client
	model          string
	embeddingModel string
}

func NewOpenAIClient(cfg *config.Config) *OpenAIClient {
	openaiConfig := openai.DefaultConfig(cfg.OpenAIKey)
	if cfg.OpenAIBaseURL != "" {
		openaiConfig.BaseURL = cfg.OpenAIBaseURL
	}
	// Fallback if empty, though config has default
	embModel := cfg.OpenAIEmbeddingModel
	if embModel == "" {
		embModel = string(openai.AdaEmbeddingV2)
	}

	return &OpenAIClient{
		client:         openai.NewClientWithConfig(openaiConfig),
		model:          cfg.OpenAIModel,
		embeddingModel: embModel,
	}
}

// GenerateText via OpenAI Chat Completion
func (c *OpenAIClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	start := time.Now()
	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: c.model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	duration := time.Since(start)
	logger.LLM(ctx, c.model, "chat_completion", duration, err)

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// EmbedQuery generates embedding for a single string
func (c *OpenAIClient) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()
	// Normalize newlines as per OpenAI recommendation for some models, though less critical for ada-002
	text = strings.ReplaceAll(text, "\n", " ")

	resp, err := c.client.CreateEmbeddings(
		ctx,
		openai.EmbeddingRequest{
			Input: []string{text},
			Model: openai.EmbeddingModel(c.embeddingModel),
		},
	)

	duration := time.Since(start)
	logger.LLM(ctx, c.embeddingModel, "embedding_query", duration, err)

	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return resp.Data[0].Embedding, nil
}

// EmbedDocuments generates embeddings for multiple strings
func (c *OpenAIClient) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	// Clean inputs
	for i := range texts {
		texts[i] = strings.ReplaceAll(texts[i], "\n", " ")
	}

	resp, err := c.client.CreateEmbeddings(
		ctx,
		openai.EmbeddingRequest{
			Input: texts,
			Model: openai.EmbeddingModel(c.embeddingModel),
		},
	)

	if err != nil {
		return nil, err
	}

	results := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		results[i] = data.Embedding
	}

	return results, nil
}
