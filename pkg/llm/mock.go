package llm

import "context"

type MockLLM struct{}

func (m *MockLLM) GenerateText(ctx context.Context, prompt string) (string, error) {
	return "Mock Summary", nil
}

func (m *MockLLM) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 1536), nil
}

func (m *MockLLM) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	return make([][]float32, len(texts)), nil
}
