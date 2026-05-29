#!/usr/bin/env bash
set -euo pipefail

# This script is the Docker ENTRYPOINT for the GitHub Action.
# It receives configuration via environment variables set by action.yml
# and executes the appropriate infra-tools command.

case "$COMMAND" in
  env-detector)
    ARGS=("--repo-root=${REPO_ROOT}")

    if [[ -n "${BASE_REF:-}" ]]; then
      ARGS+=("--base-ref=${BASE_REF}")
    fi

    if [[ -n "${PR_NUMBER:-}" ]]; then
      ARGS+=("--pr-number=${PR_NUMBER}")
    fi

    if [[ -n "${GITHUB_TOKEN:-}" ]]; then
      ARGS+=("--github-token=${GITHUB_TOKEN}")
    fi

    if [[ -n "${GITHUB_REPOSITORY:-}" ]]; then
      ARGS+=("--repo=${GITHUB_REPOSITORY}")
    fi

    if [[ -n "${OVERLAYS_DIR:-}" ]]; then
      ARGS+=("--overlays-dir=${OVERLAYS_DIR}")
    fi

    if [[ "${CLUSTER_LABELS:-false}" == "true" ]]; then
      ARGS+=("--cluster-labels")
    fi

    if [[ "${DRY_RUN:-false}" == "true" ]]; then
      ARGS+=("--dry-run")
    fi

    if [[ -n "${LOG_FILE:-}" ]]; then
      ARGS+=("--log-file=${LOG_FILE}")
    fi

    exec /usr/local/bin/env-detector "${ARGS[@]}"
    ;;

  render-diff)
    ARGS=("--repo-root=${REPO_ROOT}")

    if [[ -n "${BASE_REF:-}" ]]; then
      ARGS+=("--base-ref=${BASE_REF}")
    fi

    if [[ -n "${OUTPUT_MODE:-}" ]]; then
      ARGS+=("--output-mode=${OUTPUT_MODE}")
    fi

    if [[ -n "${OUTPUT_DIR:-}" ]]; then
      ARGS+=("--output-dir=${OUTPUT_DIR}")
    fi

    if [[ -n "${LOG_FILE:-}" ]]; then
      ARGS+=("--log-file=${LOG_FILE}")
    fi

    exec /usr/local/bin/render-diff "${ARGS[@]}"
    ;;

  *)
    echo "Error: unknown command '${COMMAND}'. Must be 'env-detector' or 'render-diff'." >&2
    exit 1
    ;;
esac
