package utils

import (
	"strings"
	"testing"
)

func TestNormalizeToKubernetesName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(string) bool
	}{
		{
			name:  "simple lowercase",
			input: "ssh",
			validate: func(result string) bool {
				return result == "ssh"
			},
		},
		{
			name:  "uppercase to lowercase",
			input: "SSH",
			validate: func(result string) bool {
				return result == "ssh"
			},
		},
		{
			name:  "spaces to dashes",
			input: "my tag name",
			validate: func(result string) bool {
				return result == "my-tag-name"
			},
		},
		{
			name:  "special characters",
			input: "tag_with@special#chars!",
			validate: func(result string) bool {
				return result == "tag-with-special-chars"
			},
		},
		{
			name:  "leading and trailing invalid chars",
			input: "___tag___",
			validate: func(result string) bool {
				return result == "tag"
			},
		},
		{
			name:  "consecutive dashes",
			input: "tag---with---dashes",
			validate: func(result string) bool {
				return result == "tag-with-dashes"
			},
		},
		{
			name:  "empty string",
			input: "",
			validate: func(result string) bool {
				return result == "unnamed"
			},
		},
		{
			name:  "only invalid chars",
			input: "!!!@@@###",
			validate: func(result string) bool {
				return strings.HasPrefix(result, "tag-") && len(result) == 12
			},
		},
		{
			name:  "very long tag",
			input: strings.Repeat("a", 300),
			validate: func(result string) bool {
				return len(result) <= 253
			},
		},
		{
			name:  "idempotent - same input produces same output",
			input: "MyTag@123",
			validate: func(result string) bool {
				first := NormalizeToKubernetesName("MyTag@123")
				second := NormalizeToKubernetesName("MyTag@123")
				return first == second && first == result
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeToKubernetesName(tt.input)
			if !tt.validate(result) {
				t.Errorf("NormalizeToKubernetesName(%q) = %q, validation failed", tt.input, result)
			}
			if len(result) > 253 {
				t.Errorf("NormalizeToKubernetesName(%q) = %q, length %d exceeds 253", tt.input, result, len(result))
			}
			if result != "" && !isValidK8sName(result) {
				t.Errorf("NormalizeToKubernetesName(%q) = %q, not a valid Kubernetes name", tt.input, result)
			}
		})
	}
}

func isValidK8sName(name string) (valid bool) {
	if len(name) == 0 || len(name) > 253 {
		return
	}
	for i, c := range name {
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		isDash := c == '-'
		isDot := c == '.'
		if !isAlphaNum && !isDash && !isDot {
			return
		}
		if i == 0 || i == len(name)-1 {
			if !isAlphaNum {
				return
			}
		}
	}
	valid = true
	return
}
