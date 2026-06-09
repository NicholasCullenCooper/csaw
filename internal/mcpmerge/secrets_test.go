package mcpmerge

import (
	"strings"
	"testing"
)

func TestValidateNoLiteralSecretsAllowsSafeForms(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{
			name: "Codex env_vars list",
			body: map[string]interface{}{
				"command":  "npx",
				"env_vars": []interface{}{"GITHUB_TOKEN"},
			},
		},
		{
			name: "Codex bearer_token_env_var (string reference)",
			body: map[string]interface{}{
				"transport":            "http",
				"url":                  "https://example.com",
				"bearer_token_env_var": "GH_PAT",
			},
		},
		{
			name: "inline env map with $VAR reference",
			body: map[string]interface{}{
				"command": "node",
				"env": map[string]interface{}{
					"API_TOKEN": "$API_TOKEN",
				},
			},
		},
		{
			name: "inline env map with ${env:VAR} reference",
			body: map[string]interface{}{
				"command": "node",
				"env": map[string]interface{}{
					"SECRET_KEY": "${env:SECRET_KEY}",
				},
			},
		},
		{
			name: "non-sensitive field is fine inline",
			body: map[string]interface{}{
				"command": "node",
				"timeout": float64(30),
			},
		},
		{
			name: "public_key passes through (excluded from _key heuristic)",
			body: map[string]interface{}{
				"command":    "node",
				"public_key": "ssh-rsa AAAA...",
			},
		},
	}
	for _, tc := range cases {
		if err := ValidateNoLiteralSecrets("x", tc.body); err != nil {
			t.Errorf("%s: unexpected validation error: %v", tc.name, err)
		}
	}
}

func TestValidateNoLiteralSecretsCatchesLiterals(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
		want string // substring expected in error
	}{
		{
			name: "literal token at top level",
			body: map[string]interface{}{
				"token": "ghp_abc123definitelyrealtoken",
			},
			want: "token",
		},
		{
			name: "literal in nested env",
			body: map[string]interface{}{
				"command": "node",
				"env": map[string]interface{}{
					"API_KEY": "sk-real-key-here-not-a-reference",
				},
			},
			want: "env.API_KEY",
		},
		{
			name: "secret field",
			body: map[string]interface{}{
				"secret": "literal-not-env-ref",
			},
			want: "secret",
		},
		{
			name: "password field",
			body: map[string]interface{}{
				"password": "literal",
			},
			want: "password",
		},
		{
			name: "bearer token literal",
			body: map[string]interface{}{
				"bearer_token": "eyJliteralJWTwouldgohere",
			},
			want: "bearer_token",
		},
	}
	for _, tc := range cases {
		err := ValidateNoLiteralSecrets("test", tc.body)
		if err == nil {
			t.Errorf("%s: expected validation error, got nil", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q should mention %q", tc.name, err.Error(), tc.want)
		}
	}
}
