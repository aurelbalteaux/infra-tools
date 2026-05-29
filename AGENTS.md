# infra-tools

Go-based tooling for analyzing infra-deployments ArgoCD structure.

## Quick Commands

| Action       | Command                           |
|--------------|-----------------------------------|
| Build        | `make build`                      |
| Test         | `make test`                       |
| Lint         | `make lint`                       |
| Fix lint     | `make lint-fix`                   |
| Build image  | `make image`                      |

## Project Structure

- `cmd/env-detector/` - Detects affected environments/clusters from file changes
- `cmd/render-diff/` - Computes kustomize render deltas
- `internal/` - Shared packages (appset, deptree, detector, git, github, kustomize, renderdiff)
- `action.yml` - GitHub Action manifest
- `entrypoint.sh` - Docker entrypoint for Action execution
- `Dockerfile` - Multi-stage container build

## Development

Prerequisites: Go 1.24+, kustomize

Tests are self-contained and can be run with `make test`. The tools operate
on git repositories and kustomize overlays - they can be tested against
infra-deployments or internal-infra-deployments.

## Packaging

The repository serves dual purpose:
1. Standalone Go binaries (via `make build`)
2. GitHub Action (via `uses: aurelbalteaux/infra-tools@main`)

Container image publishes to quay.io/aurelbalteaux/infra-tools:latest on main
branch commits via .github/workflows/publish-image.yaml.
