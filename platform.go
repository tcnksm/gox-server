package main

import "strings"

// guessPlatform detects OS and Arch which user requests
// References https://github.com/flynn/flynn-cli-redirect
func guessPlatform(userAgent string) (string, string) {
	// Handle everything as lower case string
	userAgent = strings.ToLower(userAgent)
	return guessOS(userAgent), guessArch(userAgent)
}

func guessOS(userAgent string) string {
	if isDarwin(userAgent) {
		return "darwin"
	}

	if isWindows(userAgent) {
		return "windows"
	}

	return "linux"
}

func guessArch(userAgent string) string {
	if isAmd64(userAgent) || isDarwin(userAgent) {
		return "amd64"
	}
	return "386"
}

func isDarwin(userAgent string) bool {
	return strings.Contains(userAgent, "mac os x") || strings.Contains(userAgent, "darwin")
}

func isWindows(userAgent string) bool {
	return strings.Contains(userAgent, "windows")
}

func isAmd64(userAgent string) bool {
	return strings.Contains(userAgent, "x86_64") || strings.Contains(userAgent, "amd64") || isDarwin(userAgent)
}
