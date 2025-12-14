package prompts

import (
	"fmt"
	"strings"
)

var (
	// DefaultSummarizePrompt is the built-in default for summarization.
	DefaultSummarizePrompt = "Summarize the following conversation completely regarding key facts and user preferences. Ignore casual chitchat.\n\n%s"
	// DefaultExtractProfilePrompt is for identifying user attributes.
	DefaultExtractProfilePrompt = "Analyze the following interaction. Identify any persistent user preferences, traits, or facts that should be remembered for future personalization. Return ONLY these facts as a bulleted list. If none, return 'None'.\n\n%s"
)

// Registry holds all prompts used in the application.
type Registry struct {
	Summarize      string
	ExtractProfile string
}

// NewRegistry creates a prompt registry, preferring custom overrides if provided.
func NewRegistry(customSummarize, customExtract string) *Registry {
	r := &Registry{
		Summarize:      DefaultSummarizePrompt,
		ExtractProfile: DefaultExtractProfilePrompt,
	}

	if customSummarize != "" {
		r.Summarize = strings.ReplaceAll(customSummarize, "\\n", "\n")
	}
	if customExtract != "" {
		r.ExtractProfile = strings.ReplaceAll(customExtract, "\\n", "\n")
	}

	return r
}

// GetSummarizePrompt returns the formatted summarization prompt.
func (r *Registry) GetSummarizePrompt(content string) string {
	return fmt.Sprintf(r.Summarize, content)
}

// GetExtractProfilePrompt returns the formatted profile extraction prompt.
func (r *Registry) GetExtractProfilePrompt(content string) string {
	return fmt.Sprintf(r.ExtractProfile, content)
}
