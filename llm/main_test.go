package llm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xhad/yes/llm"
)

func TestPrompt(t *testing.T) {
	tests := []struct {
		name       string
		systemRole string
		prompt     string
	}{
		{
			name:       "basic prompt test",
			systemRole: "You are a helpful assistant",
			prompt:     "Hello, how are you?",
		},
		{
			name:       "empty system role",
			systemRole: "",
			prompt:     "Test prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := llm.Prompt(tt.systemRole, tt.prompt)
			t.Logf("Response: %s", response.Choices[0].Content)
			assert.NotNil(t, response)
			assert.NotEmpty(t, response.Choices[0].Content)
		})
	}
}
