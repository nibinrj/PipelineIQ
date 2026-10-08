#!/bin/sh
# Fetch the inbound secret from the controller, then start the official agent.
# JCasC creates the node. It does not let us choose the secret. This avoids a UI click.
set -eu

: "${JENKINS_URL:?JENKINS_URL is required}"
: "${JENKINS_AGENT_NAME:?JENKINS_AGENT_NAME is required}"

fetch_secret() {
  url="${JENKINS_URL%/}/computer/${JENKINS_AGENT_NAME}/jenkins-agent.jnlp"
  body=$(curl -fsS --retry 0 -u "${JENKINS_ADMIN_USER}:${JENKINS_ADMIN_PASSWORD}" "$url") || return 1
  # The first <argument> is the secret. Later arguments are the agent name and flags.
  printf '%s' "$body" | sed 's/<argument>/\n<argument>/g' | sed -n 's/^<argument>\([^<]*\).*/\1/p' | head -n 1
}

secret=${JENKINS_SECRET:-}
if [ -z "$secret" ]; then
  : "${JENKINS_ADMIN_USER:?JENKINS_ADMIN_USER is required}"
  : "${JENKINS_ADMIN_PASSWORD:?JENKINS_ADMIN_PASSWORD is required}"
  i=0
  while [ "$i" -lt 90 ]; do
    if secret=$(fetch_secret) && [ -n "$secret" ]; then
      break
    fi
    secret=""
    i=$((i + 1))
    if [ $((i % 10)) -eq 0 ]; then
      echo "pipelineiq-agent: still waiting for the inbound secret (${i})" >&2
    fi
    sleep 2
  done
fi

if [ -z "$secret" ]; then
  echo "pipelineiq-agent: could not read the inbound secret from ${JENKINS_URL}" >&2
  exit 1
fi

# The image's jenkins-agent already adds -url and -name from these variables.
# Passing them again makes the remoting parser reject the launch.
echo "pipelineiq-agent: starting inbound WebSocket agent" >&2
export JENKINS_SECRET="$secret"
export JENKINS_WEB_SOCKET=true
exec jenkins-agent "$@"
