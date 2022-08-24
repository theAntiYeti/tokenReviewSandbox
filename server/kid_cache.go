package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func GetURL(kid string) (string, error) {
	url, err := os.ReadFile("/kid-mapping/" + kid)
	if err != nil {
		return "", fmt.Errorf("cluster not known to server: %v", err)
	}
	return string(url), nil
}

func GetClusterUrl(token string) (string, error) {
	// Extract the KID from the token
	header := strings.Split(token, ".")[0]
	headerDecode, err := b64.RawURLEncoding.DecodeString(header)
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
		return "", fmt.Errorf("no KID provided with jwt bearer token")
	}

	return GetURL(unmarshalled.Kid)
}
