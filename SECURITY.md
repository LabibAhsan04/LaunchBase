# Security policy

## Supported versions

LaunchBase is an actively developed portfolio project. Security fixes land on the default branch of this repository.

## Reporting a vulnerability

Please do **not** open a public GitHub issue for security-sensitive reports.

Email the maintainer privately with:

1. A short description of the issue
2. Steps to reproduce
3. Affected component (API, worker, Traefik, dashboard, GitHub App flow)
4. Whether you believe secrets or other users' data are exposed

We will acknowledge receipt when possible and work on a fix before any public discussion.

## Scope notes

LaunchBase runs untrusted repository builds on a single Docker host. Reports that assume perfect isolation against a hostile kernel/Docker escape are out of scope for the MVP threat model documented in `docs/security.md`. Reports that show missing documented controls (socket mounts, infra network attachment, secret leakage, auth bypass) are in scope.
