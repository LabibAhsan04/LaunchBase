# Supported frameworks

Detection reads files. It does not run the repository. npm, pnpm, and yarn run only inside the build container.

A root `Dockerfile` is used as-is. Lockfiles are ignored in that case.

Otherwise LaunchBase looks at `package.json` and the lockfile:

| Files | Result |
| --- | --- |
| `package-lock.json` | npm, `npm ci` when the lockfile is present |
| `pnpm-lock.yaml` | pnpm, `pnpm install --frozen-lockfile` |
| `yarn.lock` without `__metadata:` | Yarn classic, `yarn install --frozen-lockfile` |
| `yarn.lock` with `__metadata:` | Yarn Berry, `yarn install --immutable` |
| no lockfile | npm, `npm install` |
| more than one lockfile | error, and Docker is not started |

Framework rules, after a Dockerfile is ruled out:

| Signal | What we build |
| --- | --- |
| `next` dependency | `npx next build`, then `npx next start -H 0.0.0.0 -p 3000` |
| `vite` dependency | `npx vite build`. The published site is the `dist` directory, served by a small program LaunchBase adds. |
| `scripts.start` | `npm start`, `pnpm start`, or `yarn start`. `scripts.build` runs first when that key exists. |
| both `next` and `vite` | error |
| anything else | error |

The commands above are fixed. Values from `package.json` scripts are not copied into the Dockerfile. Those scripts still run when the fixed command is `npm run build` or `npm start`, and that happens in the container.

Generated images are multi-stage. The runtime stage runs as the `node` user. Vite's runtime image contains `dist` and the static server, not the install tools. Node and Next copy the app after production dependencies are pruned. Yarn Berry only drops `.yarn/cache`; its dev dependencies can still be in the runtime image.

Other limits:

- Vite's output directory has to be `dist`.
- One exposed port, 3000, unless you pass `--port`.
- The container root filesystem is read-only. `/tmp` is writable. Next also gets a writable `/app/.next/cache`.
- Package manager versions come from the Node 22 image's Corepack, not from the repo's `packageManager` field.
