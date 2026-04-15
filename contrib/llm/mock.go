package llm

import "context"

// MockLLM is a test double that returns a fixed response.
type MockLLM struct {
	Response     string
	Err          error
	ProviderName string
	ModelName    string
}

func (m *MockLLM) Generate(_ context.Context, _ string) (string, error) {
	return m.Response, m.Err
}

func (m *MockLLM) Provider() string {
	if m.ProviderName == "" {
		return "mock"
	}
	return m.ProviderName
}

func (m *MockLLM) Model() string {
	if m.ModelName == "" {
		return "mock-llm"
	}
	return m.ModelName
}
