//nolint:revive
package utils

import "strings"

// OCIPrefix is the standard prefix for OCI URLs.
const OCIPrefix = "oci://"

// AddOCIPrefix adds the "oci://" prefix to the given URL if it doesn't already have it.
// It returns the URL unchanged if it already starts with "oci://" (case-sensitive).
// If the input is empty, it returns an empty string.
func AddOCIPrefix(ociURL string) string {
	// Handle empty string
	if ociURL == "" {
		return ""
	}

	if strings.HasPrefix(ociURL, OCIPrefix) {
		return ociURL
	}
	return OCIPrefix + ociURL
}

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
