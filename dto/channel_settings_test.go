package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateResponseOverrideRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rules   []ResponseOverrideRule
		wantErr string
	}{
		{
			name: "unconditional rule",
			rules: []ResponseOverrideRule{
				{Match: "gpt-image-2.5-flare", Replace: "gpt-image-2.5-sunburst"},
			},
		},
		{
			name: "conditional rule",
			rules: []ResponseOverrideRule{
				{Match: "xhigh", Replace: "max", RequestParam: "quality", RequestValue: "max"},
			},
		},
		{
			name:    "empty match",
			rules:   []ResponseOverrideRule{{Replace: "max"}},
			wantErr: "match keyword is required",
		},
		{
			name:    "parameter without value",
			rules:   []ResponseOverrideRule{{Match: "xhigh", RequestParam: "quality"}},
			wantErr: "request_value is required",
		},
		{
			name:    "value without parameter",
			rules:   []ResponseOverrideRule{{Match: "xhigh", RequestValue: "max"}},
			wantErr: "request_param is required",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateResponseOverrideRules(test.rules)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name         string
		incomingPath string
	}{
		{name: "chat completions", incomingPath: "/v1/chat/completions"},
		{name: "responses compact", incomingPath: "/v1/responses/compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: tt.incomingPath,
						UpstreamPath: "/v1/chat/completions",
						Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}
