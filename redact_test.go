package agentotel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDropAllPayloadsRedactor(t *testing.T) {
	payload, err := DropAllPayloadsRedactor{}.RedactPrompt(context.Background(), PromptPayload{
		Kind:       PayloadPrompt,
		Attributes: map[string]any{"k": "v"},
		Value:      secretPrompt,
	})
	require.NoError(t, err)
	require.Nil(t, payload.Value)
	require.Nil(t, payload.Attributes)
}
