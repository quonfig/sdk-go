package quonfig

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// deriveStreamURL returns the SSE/streaming URL that corresponds to an API base
// URL. Per the two-app split (HTTP on primary.quonfig.com, SSE on
// stream.primary.quonfig.com), the derivation is mechanical: prepend "stream."
// to the hostname and leave everything else (scheme, port, path, query, user
// info) untouched.
//
// Examples:
//
//	https://primary.quonfig.com            -> https://stream.primary.quonfig.com
//	http://localhost:8080                  -> http://stream.localhost:8080
//	https://api-delivery.localhost         -> https://stream.api-delivery.localhost
//	https://primary.quonfig.com/prefix     -> https://stream.primary.quonfig.com/prefix
//
// Returns an error if the input does not parse as a URL with a host.
func deriveStreamURL(apiURL string) (string, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return "", errors.New("deriveStreamURL: empty URL")
	}
	u, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("deriveStreamURL: parse %q: %w", apiURL, err)
	}
	if u.Scheme == "" {
		return "", fmt.Errorf("deriveStreamURL: missing scheme in %q", apiURL)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("deriveStreamURL: missing host in %q", apiURL)
	}
	port := u.Port()
	newHost := "stream." + host
	if port != "" {
		u.Host = newHost + ":" + port
	} else {
		u.Host = newHost
	}
	return u.String(), nil
}
