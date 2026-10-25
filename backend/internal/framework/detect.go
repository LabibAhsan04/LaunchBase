package framework

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	Dockerfile = "dockerfile"
	Vite       = "vite"
	Next       = "nextjs"
	Node       = "node"

	NPM       = "npm"
	PNPM      = "pnpm"
	Yarn      = "yarn"
	YarnBerry = "yarn-berry"
)

// Result is what we decided from the files on disk.
// BuildCommand and StartCommand are ours. They are never copied out of package.json.
type Result struct {
	Framework      string
	PackageManager string
	BuildCommand   string
	StartCommand   string
	OutputDir      string
	Confidence     string
	Reason         string
	Tmpfs          []string
	runBuild       bool
}

type packageFile struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// Detect only reads files. It does not run the repository.
func Detect(dir string) (Result, error) {
	info, err := os.Stat(filepath.Join(dir, "Dockerfile"))
	if err == nil && !info.IsDir() {
		return Result{
			Framework:  Dockerfile,
			Confidence: "high",
			Reason:     "repository contains a Dockerfile",
		}, nil
	}
	if err == nil && info.IsDir() {
		return Result{}, fmt.Errorf("Dockerfile is a directory")
	}
	if err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}

	manager, err := lockfileManager(dir)
	if err != nil {
		return Result{}, err
	}

	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return Result{}, fmt.Errorf("unsupported project: no Dockerfile and no package.json")
		}
		return Result{}, err
	}
	var pkg packageFile
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return Result{}, fmt.Errorf("package.json: %w", err)
	}

	has := func(name string) bool {
		_, dep := pkg.Dependencies[name]
		_, dev := pkg.DevDependencies[name]
		return dep || dev
	}
	_, buildScript := pkg.Scripts["build"]
	_, startScript := pkg.Scripts["start"]

	if has("next") && has("vite") {
		return Result{}, fmt.Errorf("unsupported project: both next and vite are dependencies")
	}

	confidence := "high"
	noLock := manager == ""
	if noLock {
		manager = NPM
		confidence = "medium"
	}
	note := managerLabel(manager)
	if noLock {
		note = "no lockfile, using npm"
	}

	switch {
	case has("next"):
		return Result{
			Framework:      Next,
			PackageManager: manager,
			BuildCommand:   nextBuild,
			StartCommand:   nextStart,
			OutputDir:      ".next",
			Confidence:     confidence,
			Reason:         "next dependency, " + note,
			Tmpfs:          []string{"/app/.next/cache"},
		}, nil
	case has("vite"):
		return Result{
			Framework:      Vite,
			PackageManager: manager,
			BuildCommand:   viteBuild,
			StartCommand:   "node server.js",
			OutputDir:      "dist",
			Confidence:     confidence,
			Reason:         "vite dependency, " + note,
		}, nil
	case startScript:
		res := Result{
			Framework:      Node,
			PackageManager: manager,
			StartCommand:   startCommand(manager),
			OutputDir:      ".",
			Confidence:     confidence,
			Reason:         "node start script, " + note,
			runBuild:       buildScript,
		}
		if buildScript {
			res.BuildCommand = buildCommand(manager)
		}
		return res, nil
	default:
		return Result{}, fmt.Errorf("unsupported project: no Dockerfile, Vite app, Next.js app, or Node start script")
	}
}

func lockfileManager(dir string) (string, error) {
	type lock struct {
		name    string
		manager string
	}
	candidates := []lock{
		{"package-lock.json", NPM},
		{"pnpm-lock.yaml", PNPM},
		{"yarn.lock", Yarn},
	}
	var found []lock
	for _, c := range candidates {
		info, err := os.Stat(filepath.Join(dir, c.name))
		if err == nil && !info.IsDir() {
			found = append(found, c)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		if found[0].manager == Yarn {
			return yarnKind(filepath.Join(dir, "yarn.lock"))
		}
		return found[0].manager, nil
	default:
		names := make([]string, len(found))
		for i, f := range found {
			names[i] = f.name
		}
		sort.Strings(names)
		return "", fmt.Errorf("conflicting lockfiles: %s", strings.Join(names, ", "))
	}
}

func yarnKind(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, 4096)
	n, _ := f.Read(buf)
	if strings.Contains(string(buf[:n]), "__metadata:") {
		return YarnBerry, nil
	}
	return Yarn, nil
}

func managerLabel(manager string) string {
	switch manager {
	case NPM:
		return "npm lockfile"
	case PNPM:
		return "pnpm lockfile"
	case Yarn:
		return "yarn classic lockfile"
	case YarnBerry:
		return "yarn berry lockfile"
	default:
		return manager
	}
}

const (
	nextBuild = "npx next build"
	nextStart = "npx next start -H 0.0.0.0 -p 3000"
	viteBuild = "npx vite build"
)

func buildCommand(manager string) string {
	switch manager {
	case PNPM:
		return "pnpm run build"
	case Yarn, YarnBerry:
		return "yarn run build"
	default:
		return "npm run build"
	}
}

func startCommand(manager string) string {
	switch manager {
	case PNPM:
		return "pnpm start"
	case Yarn, YarnBerry:
		return "yarn start"
	default:
		return "npm start"
	}
}
