package dockerengine

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	DefaultRuntimeNetwork = "launchbase-runtime"
	DefaultBaseDomain     = "launchbase.localhost"
)

var nonDNS = regexp.MustCompile(`[^a-z0-9-]+`)

func RuntimeNetwork() string {
	if v := strings.TrimSpace(os.Getenv("RUNTIME_NETWORK")); v != "" {
		return v
	}
	return DefaultRuntimeNetwork
}

func BaseDomain() string {
	if v := strings.TrimSpace(os.Getenv("BASE_DOMAIN")); v != "" {
		return strings.ToLower(v)
	}
	return DefaultBaseDomain
}

// Slugify turns a project name into a DNS label (lowercase, a-z0-9-).
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, "_", "-")
	s = nonDNS.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if len(s) > 48 {
		s = strings.Trim(s[:48], "-")
	}
	if s == "" {
		s = "app"
	}
	return s
}

// DeploymentHost is the unique hostname for one deployment.
// Example: demo-7a2d4c3b.launchbase.localhost
func DeploymentHost(slug, deploymentID, domain string) string {
	slug = slugOrApp(slug)
	domain = domainOrDefault(domain)
	short := ShortID(deploymentID)
	return fmt.Sprintf("%s-%s.%s", slug, short, domain)
}

// ProjectHost is the stable hostname for the active deployment of a project.
// Example: demo.launchbase.localhost
func ProjectHost(slug, domain string) string {
	return fmt.Sprintf("%s.%s", slugOrApp(slug), domainOrDefault(domain))
}

func PublicURL(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return "http://" + host + "/"
}

// LocalhostDial rewrites *.localhost (and localhost) URLs to dial 127.0.0.1 while
// preserving the original Host header. On Linux, .localhost often resolves to ::1
// while Docker publishes host ports on IPv4 only, which breaks Traefik health checks in CI.
func LocalhostDial(rawURL string) (dialURL, hostHeader string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	hostHeader = u.Hostname()
	if hostHeader == "" {
		return rawURL, "", nil
	}
	if hostHeader != "localhost" && !strings.HasSuffix(hostHeader, ".localhost") {
		return rawURL, hostHeader, nil
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	rewritten := *u
	rewritten.Host = "127.0.0.1:" + port
	return rewritten.String(), hostHeader, nil
}

func ShortID(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	if len(id) > 8 {
		id = id[:8]
	}
	if id == "" {
		return "00000000"
	}
	return strings.ToLower(id)
}

func TraefikLabels(projectHost, deploymentHost, port string) map[string]string {
	routerProject := "lb-" + strings.ReplaceAll(projectHost, ".", "-")
	routerDeploy := "lb-" + strings.ReplaceAll(deploymentHost, ".", "-")
	return map[string]string{
		"traefik.enable":         "true",
		"traefik.docker.network": RuntimeNetwork(),

		"traefik.http.routers." + routerProject + ".rule":                      "Host(`" + projectHost + "`)",
		"traefik.http.routers." + routerProject + ".entrypoints":               "web",
		"traefik.http.routers." + routerProject + ".service":                   routerProject,
		"traefik.http.services." + routerProject + ".loadbalancer.server.port": port,

		"traefik.http.routers." + routerDeploy + ".rule":                      "Host(`" + deploymentHost + "`)",
		"traefik.http.routers." + routerDeploy + ".entrypoints":               "web",
		"traefik.http.routers." + routerDeploy + ".service":                   routerDeploy,
		"traefik.http.services." + routerDeploy + ".loadbalancer.server.port": port,
	}
}

func slugOrApp(slug string) string {
	slug = Slugify(slug)
	if slug == "" {
		return "app"
	}
	return slug
}

func domainOrDefault(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return DefaultBaseDomain
	}
	return domain
}
