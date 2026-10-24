package dockerengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// Runner shells out. Tests pass a fake so unit tests never touch Docker.
type Runner interface {
	Run(ctx context.Context, bin string, args []string, stdout, stderr io.Writer) ([]byte, error)
}

type Local struct{}

func (Local) Run(ctx context.Context, bin string, args []string, stdout, stderr io.Writer) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var buf bytes.Buffer
	if stdout != nil {
		cmd.Stdout = io.MultiWriter(&buf, stdout)
	} else {
		cmd.Stdout = &buf
	}
	if stderr != nil {
		cmd.Stderr = io.MultiWriter(&buf, stderr)
	} else {
		cmd.Stderr = &buf
	}
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(buf.String())
		if len(msg) > 500 {
			msg = msg[len(msg)-500:]
		}
		if msg != "" {
			return buf.Bytes(), fmt.Errorf("%w: %s", err, msg)
		}
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

type Client struct {
	Runner Runner
	Memory string
	CPUs   string
	PIDs   string
}

func (c Client) run(ctx context.Context, args []string, log io.Writer) ([]byte, error) {
	r := c.Runner
	if r == nil {
		r = Local{}
	}
	return r.Run(ctx, "docker", args, log, log)
}

func (c Client) Build(ctx context.Context, dir, tag string, log io.Writer) error {
	_, err := c.run(ctx, buildArgs(dir, tag), log)
	if err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	return nil
}

func buildArgs(dir, tag string) []string {
	return []string{"build", "-t", tag, dir}
}

type RunSpec struct {
	Name   string
	Image  string
	Port   string
	Memory string
	CPUs   string
	PIDs   string
	User   string
	Tmpfs  []string
}

func (c Client) Start(ctx context.Context, spec RunSpec) error {
	if spec.Memory == "" {
		spec.Memory = c.Memory
	}
	if spec.CPUs == "" {
		spec.CPUs = c.CPUs
	}
	if spec.PIDs == "" {
		spec.PIDs = c.PIDs
	}
	_, err := c.run(ctx, containerRunArgs(spec), nil)
	if err != nil {
		return fmt.Errorf("docker run: %w", err)
	}
	return nil
}

func containerRunArgs(s RunSpec) []string {
	// 127.0.0.1::port lets docker pick a free localhost port.
	// read-only root, with /tmp writable, because a lot of images still need a scratch dir.
	args := []string{
		"run", "-d",
		"--name", s.Name,
		"--memory", s.Memory,
		"--cpus", s.CPUs,
		"--pids-limit", s.PIDs,
		"--read-only",
		"--tmpfs", "/tmp",
		"-p", "127.0.0.1::" + s.Port,
	}
	for _, dir := range s.Tmpfs {
		args = append(args, "--tmpfs", dir)
	}
	if s.User != "" {
		args = append(args, "--user", s.User)
	}
	args = append(args, s.Image)
	return args
}

func (c Client) ImageUser(ctx context.Context, tag string) (string, error) {
	out, err := c.run(ctx, []string{"image", "inspect", "--format", "{{.Config.User}}", tag}, nil)
	if err != nil {
		return "", fmt.Errorf("inspect user: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c Client) ExposedPort(ctx context.Context, tag string) (int, error) {
	out, err := c.run(ctx, []string{"image", "inspect", "--format", "{{json .Config.ExposedPorts}}", tag}, nil)
	if err != nil {
		return 0, fmt.Errorf("inspect ports: %w", err)
	}
	ports, err := parseExposedPorts(string(out))
	if err != nil {
		return 0, err
	}
	switch len(ports) {
	case 0:
		return 0, fmt.Errorf("image exposes no port, pass --port")
	case 1:
		return ports[0], nil
	default:
		return 0, fmt.Errorf("image exposes %v, pass --port", ports)
	}
}

func parseExposedPorts(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var exposed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &exposed); err != nil {
		return nil, fmt.Errorf("exposed ports %q: %w", raw, err)
	}
	ports := make([]int, 0, len(exposed))
	for key := range exposed {
		num := strings.Split(key, "/")[0]
		port, err := strconv.Atoi(num)
		if err != nil {
			return nil, fmt.Errorf("exposed port %q", key)
		}
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports, nil
}

func (c Client) HostPort(ctx context.Context, name string, port int) (string, error) {
	out, err := c.run(ctx, []string{"port", name, strconv.Itoa(port) + "/tcp"}, nil)
	if err != nil {
		return "", fmt.Errorf("docker port: %w", err)
	}
	return parsePublished(string(out))
}

func parsePublished(out string) (string, error) {
	line := strings.TrimSpace(out)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	colon := strings.LastIndex(line, ":")
	if colon < 0 {
		return "", fmt.Errorf("cannot parse published port %q", out)
	}
	host := line[:colon]
	port := line[colon+1:]
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("cannot parse published port %q", out)
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return host + ":" + port, nil
}

func (c Client) RemoveContainer(ctx context.Context, name string) error {
	_, err := c.run(ctx, []string{"rm", "-f", name}, nil)
	if err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}

func (c Client) RemoveImage(ctx context.Context, tag string) error {
	_, err := c.run(ctx, []string{"image", "rm", tag}, nil)
	if err != nil {
		return fmt.Errorf("remove image: %w", err)
	}
	return nil
}

// NonRootUser returns the image user when it is not root.
func NonRootUser(user string) string {
	user = strings.TrimSpace(user)
	switch user {
	case "", "root", "0", "0:0":
		return ""
	}
	if strings.HasPrefix(user, "0:") {
		return ""
	}
	return user
}
