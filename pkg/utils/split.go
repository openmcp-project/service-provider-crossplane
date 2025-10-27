package utils

import "strings"

// SplitURLAndTag splits an OCI URL into its base URL and tag.
// Example 1: "registry.example.com/repo/image:v1" -> "registry.example.com/repo/image", "v1"
// Example 2: "registry.example.com:5000/repo/image:v1" -> "registry.example.com:5000/repo/image", "v1"
func SplitURLAndTag(ociURL string) (string, string) {
	lastColonIndex := strings.LastIndex(ociURL, ":")
	if lastColonIndex == -1 {
		// No colon found, return the original URL and an empty tag
		return ociURL, ""
	}

	potentialTag := ociURL[lastColonIndex+1:]

	// If the potential tag contains '/', it's likely part of a port/path, not a tag
	if strings.Contains(potentialTag, "/") {
		return ociURL, ""
	}

	// If we reach here, it's likely a tag
	return ociURL[:lastColonIndex], potentialTag
}
