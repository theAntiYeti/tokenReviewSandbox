package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// getURL takes the KID in the token payload and swaps it for a URL using a file (config-map) used as a dictionary.
func getURL(kid string) (string, error) {
	url, err := os.ReadFile("/kid-mapping/" + kid)
	if err != nil {
		return "", fmt.Errorf("cluster not known to server: %v", err)
	}
	return string(url), nil
}

// GetClusterUrl extracts the KID from a payload and uses it to work out the client's Kubernetes API URL.
func GetClusterUrl(payload string) (string, error) {
	// Extract the KID from the payload
	header := strings.Split(payload, ".")[0]
	headerDecode, err := b64.RawURLEncoding.DecodeString(header)

	// KID is provided by Kubernetes in token.
	if err != nil {
		return "", err
	}
	var unmarshalled struct {
		Kid string `json:"kid"`
	}

	if err := json.Unmarshal(headerDecode, &unmarshalled); err != nil {
		return "", err
	}
	if unmarshalled.Kid == "" {
		return "", fmt.Errorf("no KID provided with jwt bearer payload")
	}

	return getURL(unmarshalled.Kid)
}
