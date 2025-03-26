#!/bin/bash

set -euo pipefail

WEBHOOK_URL="${SLACK_WEB_HOOK:-}"
INPUT_JSON_FILE="${1:-result.json}"

if [[ -z "$WEBHOOK_URL" ]]; then
  echo "❌ SLACK_WEB_HOOK is not set"
  exit 1
fi

function post_to_slack() {
  local title="$1"
  local content="$2"

  if [[ -n "$content" ]]; then
    curl -s -X POST -H 'Content-type: application/json' \
      --data "$(jq -n --arg text "*$title*  \`\`\`$content\`\`\`" '{text: $text}')" \
      "$WEBHOOK_URL"
  fi
}

# Group by status using jq
FOUND=$(jq -r '.result[] | select(.status == "FOUND_JOB_STATUS") | "- " + .job' "$INPUT_JSON_FILE")
STALE=$(jq -r '.result[] | select(.status == "STALE_JOB_STATUS") | "- " + .job' "$INPUT_JSON_FILE")
MISSING=$(jq -r '.result[] | select(.status == "MISSING_JOB_STATUS") | "- " + .job' "$INPUT_JSON_FILE")

post_to_slack "🟢 Found Jobs" "$FOUND"
post_to_slack "🟡 Stale Jobs $2" "$STALE"
post_to_slack "🔴 Missing Jobs $2" "$MISSING"

