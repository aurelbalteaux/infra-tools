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

    if [[ "${ENFORCE_RING_DEPLOYMENT:-false}" == "true" ]]; then
      ARGS+=("--enforce-ring-deployment")
    fi

    if [[ -n "${RING_REPORT_FILE:-}" ]]; then
      ARGS+=("--ring-report-file=${RING_REPORT_FILE}")
    fi

    if [[ -n "${LOG_FILE:-}" ]]; then
      ARGS+=("--log-file=${LOG_FILE}")
    fi

    # Run env-detector once and capture JSON output if needed
    if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
      # Run with --json to get structured output
      JSON_OUTPUT=$(/usr/local/bin/env-detector "${ARGS[@]}" --json 2>&1)
      EXIT_CODE=$?

      # Validate exit code
      if [[ $EXIT_CODE -ne 0 ]]; then
        echo "Error: env-detector failed with exit code $EXIT_CODE" >&2
        echo "$JSON_OUTPUT" >&2
        exit $EXIT_CODE
      fi

      # Extract JSON portion (everything from first '{' to last '}')
      # This handles multi-line JSON after INFO logs
      JSON_PORTION=$(echo "$JSON_OUTPUT" | sed -n '/{/,/}/p')

      # Validate JSON
      if ! echo "$JSON_PORTION" | jq empty 2>/dev/null; then
        echo "Error: env-detector did not produce valid JSON" >&2
        echo "$JSON_OUTPUT" >&2
        exit 1
      fi

      # Parse and write to GITHUB_OUTPUT (use compact format)
      echo "affected-environments=$(echo "$JSON_PORTION" | jq -c '.affected_environments')" >> "$GITHUB_OUTPUT"
      echo "affected-clusters=$(echo "$JSON_PORTION" | jq -c '.affected_clusters')" >> "$GITHUB_OUTPUT"
      echo "labels=$(echo "$JSON_PORTION" | jq -c '.labels')" >> "$GITHUB_OUTPUT"

      # Print the full output (including logs) to stdout for GitHub Actions logs
      echo "$JSON_OUTPUT"

      exit $EXIT_CODE
    else
      # No GITHUB_OUTPUT, just run normally
      exec /usr/local/bin/env-detector "${ARGS[@]}"
    fi
    ;;

  render-diff)
    ARGS=("--repo-root=${REPO_ROOT}")

    if [[ -n "${BASE_REF:-}" ]]; then
      ARGS+=("--base-ref=${BASE_REF}")
    fi

    if [[ -n "${OVERLAYS_DIR:-}" ]]; then
      ARGS+=("--overlays-dir=${OVERLAYS_DIR}")
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
