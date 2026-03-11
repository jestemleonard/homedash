package discovery

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/jestemleonard/homedash/internal/config"
)

const dockerSocket = "/var/run/docker.sock"

// IntegrationInfo is the subset of integration data needed for discovery.
// This avoids an import cycle with the engine package.
type IntegrationInfo struct {
	ID              string
	Images          []string
	DefaultPort     int
	ConfigDiscovery *ConfigDiscovery
}

// Discovery scans running Docker containers and matches them to integration definitions.
type Discovery struct {
	client *client.Client
}

// New creates a Discovery instance connected to the Docker socket.
// Returns nil, nil if the socket is unavailable (graceful degradation).
func New() (*Discovery, error) {
	if _, err := os.Stat(dockerSocket); err != nil {
		slog.Info("docker socket not available, skipping auto-discovery")
		return nil, nil
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://"+dockerSocket),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		cli.Close()
		slog.Warn("docker socket not responding, skipping auto-discovery", "error", err)
		return nil, nil
	}

	return &Discovery{client: cli}, nil
}

// Discover lists running containers, matches them against integration definitions,
// and returns a map of auto-discovered service configurations.
// When hostname is non-empty, the host portion of discovered URLs is replaced
// (keeping the port), e.g. http://172.26.0.1:8989 → http://192.168.0.85:8989.
func (d *Discovery) Discover(integrations []IntegrationInfo, hostname string) (map[string]config.ServiceConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containers, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	slog.Info("docker containers found", "count", len(containers))

	discovered := make(map[string]config.ServiceConfig)

	for _, info := range integrations {
		if len(info.Images) == 0 {
			continue
		}

		for _, ctr := range containers {
			if !matchImage(ctr.Image, info.Images) {
				continue
			}

			slog.Info("matched container to integration",
				"integration", info.ID,
				"container", ctr.Names,
				"image", ctr.Image,
			)

			rawURL := extractURL(ctr.Ports, info.DefaultPort)
			if rawURL == "" {
				slog.Warn("no suitable port found for container",
					"integration", info.ID,
					"image", ctr.Image,
				)
				continue
			}

			if hostname != "" {
				rawURL = replaceHost(rawURL, hostname)
			}

			svc := config.ServiceConfig{URL: rawURL, ContainerID: ctr.ID}

			if info.ConfigDiscovery != nil {
				apiKey, err := d.extractAPIKeyViaExec(ctx, ctr.ID, info.ConfigDiscovery)
				if err != nil {
					slog.Warn("could not extract API key via docker exec",
						"integration", info.ID,
						"error", err,
					)
				} else if apiKey != "" {
					svc.APIKey = apiKey
					slog.Info("discovered API key", "integration", info.ID)
				}
			}

			discovered[info.ID] = svc
			break // first matching container wins
		}
	}

	return discovered, nil
}

// replaceHost replaces only the host portion of a URL, preserving the port.
func replaceHost(rawURL, hostname string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	_, port, _ := net.SplitHostPort(u.Host)
	if port != "" {
		u.Host = net.JoinHostPort(hostname, port)
	} else {
		u.Host = hostname
	}
	return u.String()
}

// Close releases the Docker client resources.
func (d *Discovery) Close() {
	if d.client != nil {
		d.client.Close()
	}
}

// ContainerStart starts a stopped Docker container.
func (d *Discovery) ContainerStart(ctx context.Context, containerID string) error {
	return d.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

// ContainerStop stops a running Docker container.
func (d *Discovery) ContainerStop(ctx context.Context, containerID string) error {
	return d.client.ContainerStop(ctx, containerID, container.StopOptions{})
}

// extractAPIKeyViaExec reads a config file from inside a running container
// using docker exec, then parses it to extract the API key.
func (d *Discovery) extractAPIKeyViaExec(ctx context.Context, containerID string, disc *ConfigDiscovery) (string, error) {
	execCfg := container.ExecOptions{
		Cmd:          []string{"cat", disc.ConfigPath},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := d.client.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return "", fmt.Errorf("creating exec: %w", err)
	}

	resp, err := d.client.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("attaching exec: %w", err)
	}
	defer resp.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Reader); err != nil {
		return "", fmt.Errorf("reading exec output: %w", err)
	}

	// Check exit code
	inspect, err := d.client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return "", fmt.Errorf("inspecting exec: %w", err)
	}
	if inspect.ExitCode != 0 {
		return "", fmt.Errorf("cat %s exited with code %d", disc.ConfigPath, inspect.ExitCode)
	}

	// Docker multiplexed stream may include 8-byte header frames.
	// Strip them to get clean file content.
	data := stripDockerStreamHeaders(buf.Bytes())

	return parseAPIKey(data, disc)
}

// stripDockerStreamHeaders removes Docker multiplexed stream header frames.
// Each frame has an 8-byte header: [type(1), 0, 0, 0, size(4)].
// If the data doesn't look like a multiplexed stream, return it as-is.
func stripDockerStreamHeaders(raw []byte) []byte {
	if len(raw) < 8 {
		return raw
	}

	// Check if first byte is a valid stream type (stdin=0, stdout=1, stderr=2)
	if raw[0] > 2 {
		return raw // not a multiplexed stream
	}

	var clean bytes.Buffer
	pos := 0
	for pos+8 <= len(raw) {
		streamType := raw[pos]
		if streamType > 2 {
			// Not a header — append remaining data as-is
			clean.Write(raw[pos:])
			break
		}

		size := int(raw[pos+4])<<24 | int(raw[pos+5])<<16 | int(raw[pos+6])<<8 | int(raw[pos+7])
		pos += 8

		if pos+size > len(raw) {
			clean.Write(raw[pos:])
			break
		}

		if streamType == 1 { // stdout only
			clean.Write(raw[pos : pos+size])
		}
		pos += size
	}

	return clean.Bytes()
}

// extractURL finds the host-mapped port for a container.
func extractURL(ports []container.Port, defaultPort int) string {
	hostIP := hostGatewayIP()

	// First try to find a host-mapped port matching the default port
	for _, p := range ports {
		if int(p.PrivatePort) == defaultPort && p.PublicPort > 0 {
			host := p.IP
			if host == "" || host == "0.0.0.0" || host == "::" {
				host = hostIP
			}
			return fmt.Sprintf("http://%s", net.JoinHostPort(host, strconv.Itoa(int(p.PublicPort))))
		}
	}

	// Fall back to any host-mapped port
	for _, p := range ports {
		if p.PublicPort > 0 {
			host := p.IP
			if host == "" || host == "0.0.0.0" || host == "::" {
				host = hostIP
			}
			return fmt.Sprintf("http://%s", net.JoinHostPort(host, strconv.Itoa(int(p.PublicPort))))
		}
	}

	return ""
}

// hostGatewayIP returns the default gateway IP, which is the Docker host
// when running inside a container. Falls back to "localhost" if detection fails.
func hostGatewayIP() string {
	// Try reading the default gateway from /proc/net/route (Linux)
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "localhost"
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// Default route has destination 00000000
		if fields[1] != "00000000" {
			continue
		}
		// Gateway is in hex, little-endian (on Linux)
		gw, err := strconv.ParseUint(fields[2], 16, 32)
		if err != nil {
			continue
		}
		ip := net.IPv4(byte(gw), byte(gw>>8), byte(gw>>16), byte(gw>>24))
		return ip.String()
	}

	return "localhost"
}
