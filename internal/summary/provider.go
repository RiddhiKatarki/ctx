package summary

import (
	"fmt"

	"github.com/context-handoff/ctx/pkg/types"
)

// SummaryProvider is the interface for generating project summaries.
// Implementations may use templates, LLMs (OpenAI, Anthropic, Venice,
// Ollama), or local models.
type SummaryProvider interface {
	Summarize(snapshot types.Snapshot) (*types.Summary, error)
}

// Options configures which summary provider to use.
type Options struct {
	// Provider selects the implementation: "template" (default) or "openai".
	Provider string

	// LLM configuration (used when Provider is "openai" or similar).
	APIKey    string
	APIBaseURL string
	Model     string
}

// NewSummaryProvider selects an implementation based on the given options.
func NewSummaryProvider(opts Options) (SummaryProvider, error) {
	switch opts.Provider {
	case "", "template":
		return NewTemplateProvider(), nil
	case "openai":
		if opts.APIKey == "" {
			return nil, fmt.Errorf("openai summary provider requires --api-key")
		}
		baseURL := opts.APIBaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		model := opts.Model
		if model == "" {
			model = "gpt-4o"
		}
		return NewOpenAIProvider(opts.APIKey, baseURL, model), nil
	default:
		return nil, fmt.Errorf("unknown summary provider: %q", opts.Provider)
	}
}
