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
