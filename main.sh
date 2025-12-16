#!/usr/bin/env bash
set -euo pipefail

# Helper functions
get_input() {
    local name="$1"
    local default="${2:-}"
    local var_name="INPUT_${name}"
    echo "${!var_name:-$default}"
}

get_bool_input() {
    local name="$1"
    local default="${2:-false}"
    local value
    value=$(get_input "$name" "$default")
    case "${value,,}" in
        true|yes|1|on) echo "true" ;;
        *) echo "false" ;;
    esac
}

# Read inputs
INPUT_PATH=$(get_input "PATH" ".")
INPUT_OUTPUT=$(get_input "OUTPUT" "uml.json")
INPUT_PROJECT_NAME=$(get_input "PROJECT_NAME" "")
INPUT_EXCLUDE=$(get_input "EXCLUDE" ".git,.idea,.vscode,node_modules,testdata,vendor")
INPUT_INCLUDE_TESTS=$(get_bool_input "INCLUDE_TESTS" "false")
INPUT_INCLUDE_GENERATED=$(get_bool_input "INCLUDE_GENERATED" "false")
INPUT_PRETTY=$(get_bool_input "PRETTY" "true")
INPUT_UPLOAD_URL=$(get_input "UPLOAD_URL" "${DC_UPLOAD_URL:-}")
INPUT_TOKEN=$(get_input "TOKEN" "${DC_TOKEN:-}")
INPUT_SYSTEM_ELEMENT_ID=$(get_input "SYSTEM_ELEMENT_ID" "${DC_SYSTEM_ELEMENT_ID:-}")
INPUT_DRY_RUN=$(get_bool_input "DRY_RUN" "true")

# Resolve paths
WORKSPACE="${GITHUB_WORKSPACE:-.}"
ACTION_PATH="${GITHUB_ACTION_PATH:-$(dirname "$0")}"

if [[ "$INPUT_PATH" == /* ]]; then
    ANALYSIS_PATH="$INPUT_PATH"
else
    ANALYSIS_PATH="$WORKSPACE/$INPUT_PATH"
fi

if [[ "$INPUT_OUTPUT" == /* ]]; then
    OUTPUT_PATH="$INPUT_OUTPUT"
else
    OUTPUT_PATH="$WORKSPACE/$INPUT_OUTPUT"
fi

echo "Bearing UML Analyzer for Go"
echo "==========================="
echo "Analysis path: $ANALYSIS_PATH"
echo "Output file: $OUTPUT_PATH"
echo "Exclude dirs: $INPUT_EXCLUDE"
echo "Include tests: $INPUT_INCLUDE_TESTS"
echo "Include generated: $INPUT_INCLUDE_GENERATED"
echo "Pretty print: $INPUT_PRETTY"
echo ""

# Build command arguments
CMD_ARGS=()

if [[ "$INPUT_PRETTY" == "true" ]]; then
    CMD_ARGS+=("-indent" "  ")
else
    CMD_ARGS+=("-indent" "")
fi

if [[ "$INPUT_INCLUDE_TESTS" == "true" ]]; then
    CMD_ARGS+=("-tests")
fi

if [[ "$INPUT_INCLUDE_GENERATED" == "true" ]]; then
    CMD_ARGS+=("-generated")
fi

if [[ -n "$INPUT_EXCLUDE" ]]; then
    CMD_ARGS+=("-exclude" "$INPUT_EXCLUDE")
fi

CMD_ARGS+=("-o" "$OUTPUT_PATH")
CMD_ARGS+=("$ANALYSIS_PATH")

# Run the analyzer
echo "Running: bearing-go ${CMD_ARGS[*]}"
"$ACTION_PATH/bearing-go" "${CMD_ARGS[@]}"

echo ""
echo "Generated UML JSON: $OUTPUT_PATH"

# Set output
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "output-file=$OUTPUT_PATH" >> "$GITHUB_OUTPUT"
fi

# Debug: show upload configuration
echo ""
echo "Upload configuration:"
echo "  Upload URL: ${INPUT_UPLOAD_URL:-(not set)}"
echo "  Token: ${INPUT_TOKEN:+[provided]}"
echo "  System Element ID: ${INPUT_SYSTEM_ELEMENT_ID:-(not set)}"
echo "  Dry-run: $INPUT_DRY_RUN"

# Get git info for upload
get_git_info() {
    local commit_id=""
    local branch=""

    if [[ -n "${GITHUB_SHA:-}" ]]; then
        commit_id="$GITHUB_SHA"
    elif command -v git &> /dev/null && git rev-parse --git-dir &> /dev/null; then
        commit_id=$(git rev-parse HEAD 2>/dev/null || echo "")
    fi

    if [[ -n "${GITHUB_REF_NAME:-}" ]]; then
        branch="$GITHUB_REF_NAME"
    elif [[ -n "${GITHUB_HEAD_REF:-}" ]]; then
        branch="$GITHUB_HEAD_REF"
    elif command -v git &> /dev/null && git rev-parse --git-dir &> /dev/null; then
        branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
    fi

    echo "$commit_id|$branch"
}

# Handle upload
if [[ -z "$INPUT_UPLOAD_URL" ]]; then
    echo ""
    echo "Skipping upload: No upload URL provided."
    echo "To enable upload, set 'upload-url' input or DC_UPLOAD_URL environment variable."
elif [[ -n "$INPUT_UPLOAD_URL" ]]; then
    echo ""
    echo "Preparing upload..."

    GIT_INFO=$(get_git_info)
    COMMIT_ID="${GIT_INFO%%|*}"
    BRANCH="${GIT_INFO##*|}"

    # Build curl command
    CURL_CMD="curl -X POST"
    CURL_CMD+=" -H 'Accept: application/json'"

    if [[ -n "$INPUT_TOKEN" ]]; then
        CURL_CMD+=" -H 'Authorization: Bearer $INPUT_TOKEN'"
    fi

    CURL_CMD+=" -F 'file=@$OUTPUT_PATH;type=application/json'"

    if [[ -n "$INPUT_PROJECT_NAME" ]]; then
        CURL_CMD+=" -F 'projectName=$INPUT_PROJECT_NAME'"
    fi

    if [[ -n "$INPUT_SYSTEM_ELEMENT_ID" ]]; then
        CURL_CMD+=" -F 'systemElementId=$INPUT_SYSTEM_ELEMENT_ID'"
    fi

    if [[ -n "$COMMIT_ID" ]]; then
        CURL_CMD+=" -F 'commitId=$COMMIT_ID'"
    fi

    if [[ -n "$BRANCH" ]]; then
        CURL_CMD+=" -F 'branch=$BRANCH'"
    fi

    CURL_CMD+=" '$INPUT_UPLOAD_URL'"

    if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
        # Mask the token in the curl command for output
        CURL_CMD_MASKED="${CURL_CMD//$INPUT_TOKEN/***}"
        echo "curl-command=$CURL_CMD_MASKED" >> "$GITHUB_OUTPUT"
    fi

    if [[ "$INPUT_DRY_RUN" == "true" ]]; then
        echo ""
        echo "Dry-run mode: The following curl command would be executed:"
        echo "$CURL_CMD"
    else
        echo ""
        echo "Uploading to $INPUT_UPLOAD_URL..."
        eval "$CURL_CMD"
    fi
fi

echo ""
echo "Done!"
