package utils

import "strings"

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
