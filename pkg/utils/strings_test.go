package utils

import "testing"

func TestAddOCIPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "URL without prefix",
			input:    "example.com/repo",
			expected: "oci://example.com/repo",
		},
		{
			name:     "URL with oci prefix already",
			input:    "oci://example.com/repo",
			expected: "oci://example.com/repo",
		},
		{
			name:     "URL with oci prefix and path",
			input:    "oci://registry.example.com/namespace/repository:tag",
			expected: "oci://registry.example.com/namespace/repository:tag",
		},
		{
			name:     "simple repository name",
			input:    "myrepo",
			expected: "oci://myrepo",
		},
		{
			name:     "registry with port",
			input:    "localhost:5000/myrepo",
			expected: "oci://localhost:5000/myrepo",
		},
		{
			name:     "registry with port and oci prefix",
			input:    "oci://localhost:5000/myrepo",
			expected: "oci://localhost:5000/myrepo",
		},
		{
			name:     "URL with tag",
			input:    "example.com/repo:v1.0.0",
			expected: "oci://example.com/repo:v1.0.0",
		},
		{
			name:     "URL with digest",
			input:    "example.com/repo@sha256:abc123",
			expected: "oci://example.com/repo@sha256:abc123",
		},
		{
			name:     "case sensitive - uppercase OCI",
			input:    "OCI://example.com/repo",
			expected: "oci://OCI://example.com/repo",
		},
		{
			name:     "partial prefix match",
			input:    "oci:/example.com/repo",
			expected: "oci://oci:/example.com/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddOCIPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("AddOCIPrefix(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
