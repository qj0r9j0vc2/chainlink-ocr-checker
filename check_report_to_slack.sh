#!/bin/bash

set -euo pipefail

WEBHOOK_URL="${SLACK_WEB_HOOK:-}"
INPUT_JSON_FILE="${1:-result.json}"
ALERT_TITLE="${2:-Chainlink Job Alert 🚨}"

if [[ -z "$WEBHOOK_URL" ]]; then
  echo "❌ SLACK_WEB_HOOK is not set"
  exit 1
fi

function post_to_slack() {
  local title="$1"
  local content="$2"

  if [[ -n "$content" ]]; then
    local payload
    payload=$(jq -n --arg text "*${title}*

    \`\`\`
    ${content}
    \`\`\`" '{text: $text}')
    echo "📤 Posting message to Slack:"
    echo "$payload" | jq .
    curl -s -X POST -H 'Content-type: application/json' --data "$payload" "$WEBHOOK_URL"
  fi
}

function extract_jobs() {
  local status="$1"
  jq -r --arg status "$status" '.result[] | select(.status == $status) | "- " + .job' "$INPUT_JSON_FILE"
}

# Extract job statuses
FOUND=$(extract_jobs "FOUND_JOB_STATUS")
STALE=$(extract_jobs "STALE_JOB_STATUS")
MISSING=$(extract_jobs "MISSING_JOB_STATUS")
NO_ACTIVE=$(extract_jobs "NO_ACTIVE_JOB_STATUS")
ERROR=$(extract_jobs "ERROR_JOB_STATUS")

# Count jobs
FOUND_COUNT=$(echo "$FOUND" | grep -c '^-' || true)
STALE_COUNT=$(echo "$STALE" | grep -c '^-' || true)
MISSING_COUNT=$(echo "$MISSING" | grep -c '^-' || true)
NO_ACTIVE_COUNT=$(echo "$NO_ACTIVE" | grep -c '^-' || true)
ERROR_COUNT=$(echo "$ERROR" | grep -c '^-' || true)

# Final message body
SLACK_MESSAGE=$(cat <<EOF

Time: KST $(TZ=Asia/Seoul date '+%Y-%m-%d %H:%M:%S')

🟢 Found Jobs: ${FOUND_COUNT}

🟡 Stale Jobs: ${STALE_COUNT}
$STALE

🔴 Missing Jobs: ${MISSING_COUNT}
$MISSING

🔒 No Active Jobs: ${NO_ACTIVE_COUNT}
$NO_ACTIVE

🚨 Error Jobs: ${ERROR_COUNT}
$ERROR
EOF
)

post_to_slack "$ALERT_TITLE" "$SLACK_MESSAGE"