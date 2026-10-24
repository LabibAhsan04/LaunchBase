package framework

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed static/server.js
var staticServer []byte

// Our ignore list. The clone is disposable, so generated builds can replace a repo .dockerignore.
const dockerIgnore = `.git
**/.git
**/.env
**/.env.*
**/node_modules
**/.pnpm-store
**/.yarn/cache
**/.next/cache
**/npm-debug.log*
`

// WriteBuildFiles adds a Dockerfile we control. Repository script text is not written into it.
func WriteBuildFiles(dir string, res Result) error {
	if res.Framework == Dockerfile {
		return nil
	}
	body, err := renderDockerfile(res)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(body), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ".dockerignore"), []byte(dockerIgnore), 0o644); err != nil {
		return err
	}
	if res.Framework == Vite {
		if err := os.WriteFile(filepath.Join(dir, "launchbase-static-server.js"), staticServer, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderDockerfile(res Result) (string, error) {
	install, err := installLine(res.PackageManager)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("FROM node:22-alpine AS build\n")
	b.WriteString("WORKDIR /app\n")
	b.WriteString("COPY . .\n")
	b.WriteString(install)
	switch res.Framework {
	case Vite:
		if res.BuildCommand != viteBuild {
			return "", fmt.Errorf("refusing unexpected vite build command")
		}
		b.WriteString("RUN npx vite build\n")
		b.WriteString("\nFROM node:22-alpine\n")
		b.WriteString("WORKDIR /app\n")
		b.WriteString("ENV HOME=/tmp\n")
		b.WriteString("ENV PORT=3000\n")
		b.WriteString("COPY --from=build /app/dist ./dist\n")
		b.WriteString("COPY launchbase-static-server.js ./server.js\n")
		b.WriteString("USER node\n")
		b.WriteString("EXPOSE 3000\n")
		b.WriteString("CMD [\"node\", \"server.js\"]\n")
	case Next:
		if res.BuildCommand != nextBuild || res.StartCommand != nextStart {
			return "", fmt.Errorf("refusing unexpected next command")
		}
		b.WriteString("RUN npx next build\n")
		b.WriteString(pruneLine(res.PackageManager))
		b.WriteString("\nFROM node:22-alpine\n")
		b.WriteString("WORKDIR /app\n")
		b.WriteString("ENV HOME=/tmp\n")
		b.WriteString("ENV PORT=3000\n")
		b.WriteString("ENV NODE_ENV=production\n")
		b.WriteString("COPY --from=build /app ./\n")
		b.WriteString("USER node\n")
		b.WriteString("EXPOSE 3000\n")
		b.WriteString("CMD [\"npx\", \"next\", \"start\", \"-H\", \"0.0.0.0\", \"-p\", \"3000\"]\n")
	case Node:
		if res.runBuild {
			line, err := nodeBuildLine(res)
			if err != nil {
				return "", err
			}
			b.WriteString(line)
		}
		if res.StartCommand != startCommand(res.PackageManager) {
			return "", fmt.Errorf("refusing unexpected node start command")
		}
		b.WriteString(pruneLine(res.PackageManager))
		b.WriteString("\nFROM node:22-alpine\n")
		b.WriteString("WORKDIR /app\n")
		b.WriteString("ENV HOME=/tmp\n")
		b.WriteString("ENV PORT=3000\n")
		b.WriteString("COPY --from=build /app ./\n")
		b.WriteString("USER node\n")
		b.WriteString("EXPOSE 3000\n")
		b.WriteString("CMD " + cmdJSON(res.PackageManager) + "\n")
	default:
		return "", fmt.Errorf("no template for %s", res.Framework)
	}
	return b.String(), nil
}

func installLine(manager string) (string, error) {
	switch manager {
	case NPM:
		return "RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi\n", nil
	case PNPM:
		return "RUN corepack enable && corepack pnpm install --frozen-lockfile\n", nil
	case Yarn:
		return "RUN corepack enable && corepack yarn install --frozen-lockfile\n", nil
	case YarnBerry:
		return "RUN corepack enable && corepack yarn install --immutable\n", nil
	default:
		return "", fmt.Errorf("unknown package manager %q", manager)
	}
}

func pruneLine(manager string) string {
	switch manager {
	case PNPM:
		return "RUN corepack pnpm prune --prod\n"
	case Yarn:
		return "RUN corepack yarn install --frozen-lockfile --production --ignore-scripts && corepack yarn cache clean\n"
	case YarnBerry:
		return "RUN rm -rf .yarn/cache\n"
	default:
		return "RUN npm prune --omit=dev && npm cache clean --force\n"
	}
}

func nodeBuildLine(res Result) (string, error) {
	want := buildCommand(res.PackageManager)
	if res.BuildCommand != want {
		return "", fmt.Errorf("refusing unexpected node build command")
	}
	switch res.PackageManager {
	case PNPM:
		return "RUN pnpm run build\n", nil
	case Yarn, YarnBerry:
		return "RUN yarn run build\n", nil
	default:
		return "RUN npm run build\n", nil
	}
}

func cmdJSON(manager string) string {
	switch manager {
	case PNPM:
		return "[\"corepack\", \"pnpm\", \"start\"]"
	case Yarn, YarnBerry:
		return "[\"corepack\", \"yarn\", \"start\"]"
	default:
		return "[\"npm\", \"start\"]"
	}
}
