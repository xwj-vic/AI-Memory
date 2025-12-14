package llm

import "context"

// LLM defines the interface for Large Language Model text generation.
type LLM interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
}
