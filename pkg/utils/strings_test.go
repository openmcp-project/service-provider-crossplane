package utils

import "testing"

func TestSplitURLAndTag(t *testing.T) {
	testCases := []struct {
		desc    string
		ociURL  string
		wantURL string
		wantTag string
	}{
		{
			desc:    "standard URL with tag",
			ociURL:  "registry.example.com/repo/image:v1.0.0",
			wantURL: "registry.example.com/repo/image",
			wantTag: "v1.0.0",
		},
		{
			desc:    "URL with port and tag",
			ociURL:  "registry.example.com:5000/repo/image:v1.0.0",
			wantURL: "registry.example.com:5000/repo/image",
			wantTag: "v1.0.0",
		},
		{
			desc:    "URL without tag",
			ociURL:  "registry.example.com/repo/image",
			wantURL: "registry.example.com/repo/image",
			wantTag: "",
		},
		{
			desc:    "URL with port but no tag (bug case)",
			ociURL:  "registry.example.com:5000/repo/image",
			wantURL: "registry.example.com:5000/repo/image",
			wantTag: "",
		},
		{
			desc:    "URL with multiple colons in path",
			ociURL:  "registry.example.com:5000/repo:special/image:v1.0.0",
			wantURL: "registry.example.com:5000/repo:special/image",
			wantTag: "v1.0.0",
		},
		{
			desc:    "URL with digest-like tag",
			ociURL:  "registry.example.com/repo/image:sha256-abc123",
			wantURL: "registry.example.com/repo/image",
			wantTag: "sha256-abc123",
		},
		{
			desc:    "Empty string",
			ociURL:  "",
			wantURL: "",
			wantTag: "",
		},
		{
			desc:    "Just a colon",
			ociURL:  ":",
			wantURL: "",
			wantTag: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			gotURL, gotTag := SplitURLAndTag(tc.ociURL)
			if gotURL != tc.wantURL {
				t.Errorf("expected URL %q, got %q", tc.wantURL, gotURL)
			}
			if gotTag != tc.wantTag {
				t.Errorf("expected tag %q, got %q", tc.wantTag, gotTag)
			}
		})
	}
}

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
