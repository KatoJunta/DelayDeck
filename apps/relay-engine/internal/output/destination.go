package output

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const defaultRTMPPort = "1935"

type Destination struct {
	Scheme    string
	HostPort  string
	App       string
	StreamKey string
	TCURL     string
}

func ParseDestination(serverURL, streamKey string) (Destination, error) {
	serverURL = strings.TrimSpace(serverURL)
	streamKey = strings.TrimSpace(streamKey)
	if serverURL == "" {
		return Destination{}, fmt.Errorf("output server URL is empty")
	}
	if streamKey == "" {
		return Destination{}, fmt.Errorf("output stream key is empty")
	}

	parsed, err := url.Parse(serverURL)
	if err != nil {
		return Destination{}, fmt.Errorf("parse output URL: %w", err)
	}
	if parsed.Scheme != "rtmp" && parsed.Scheme != "rtmps" {
		return Destination{}, fmt.Errorf("output URL scheme must be rtmp or rtmps")
	}
	if parsed.Host == "" {
		return Destination{}, fmt.Errorf("output URL host is empty")
	}

	app := strings.Trim(strings.TrimPrefix(parsed.Path, "/"), "/")
	if app == "" {
		return Destination{}, fmt.Errorf("output URL must include an application path (e.g. /live2)")
	}

	hostPort := parsed.Host
	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		hostPort = net.JoinHostPort(hostPort, defaultRTMPPort)
	}

	tcURL := fmt.Sprintf("%s://%s/%s", parsed.Scheme, parsed.Host, app)
	if parsed.Port() != "" {
		tcURL = fmt.Sprintf("%s://%s:%s/%s", parsed.Scheme, parsed.Hostname(), parsed.Port(), app)
	}

	return Destination{
		Scheme:    parsed.Scheme,
		HostPort:  hostPort,
		App:       app,
		StreamKey: streamKey,
		TCURL:     tcURL,
	}, nil
}
