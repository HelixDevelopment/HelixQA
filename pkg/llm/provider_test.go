// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		msg     Message
		wantErr bool
		errContains string
	}{
		{
			name:    "valid user message",
			msg:     Message{Role: RoleUser, Content: "Hello"},
			wantErr: false,
		},
		{
			name:    "valid system message",
			msg:     Message{Role: RoleSystem, Content: "You are a QA agent"},
			wantErr: false,
		},
		{
			name:        "empty role",
			msg:         Message{Role: "", Content: "Hello"},
			wantErr:     true,
			errContains: "role",
		},
		{
			name:        "empty content",
			msg:         Message{Role: RoleUser, Content: ""},
			wantErr:     true,
			errContains: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResponse_HasContent(t *testing.T) {
	tests := []struct {
		name    string
		resp    Response
		want    bool
	}{
		{
			name: "with content",
			resp: Response{Content: "Analysis complete."},
			want: true,
		},
		{
			name: "empty content",
			resp: Response{Content: ""},
			want: false,
		},
		{
			name: "whitespace only",
			resp: Response{Content: "   \t\n  "},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.resp.HasContent())
		})
	}
}

func TestProviderConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         ProviderConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "valid anthropic",
			cfg: ProviderConfig{
				Name:   ProviderAnthropic,
				APIKey: "sk-ant-test-key",
				Model:  "claude-opus-4-5",
			},
			wantErr: false,
		},
		{
			name: "valid ollama",
			cfg: ProviderConfig{
				Name:    ProviderOllama,
				BaseURL: "http://localhost:11434",
				Model:   "llava",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			cfg: ProviderConfig{
				APIKey: "sk-test",
				Model:  "some-model",
			},
			wantErr:     true,
			errContains: "name",
		},
		{
			name: "anthropic missing key",
			cfg: ProviderConfig{
				Name:  ProviderAnthropic,
				Model: "claude-opus-4-5",
			},
			wantErr:     true,
			errContains: "api_key",
		},
		{
			name: "ollama missing url",
			cfg: ProviderConfig{
				Name:  ProviderOllama,
				Model: "llava",
			},
			wantErr:     true,
			errContains: "base_url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
