#!/usr/bin/env bash
# Deploys this project as the Docker Swarm stack "{{STACK_NAME}}". Runs
# anywhere — CI or your machine — and talks to the swarm manager's Docker
# over SSH using the files in this repository, so nothing needs copying to
# the server first.
#
# Environment:
#   DEPLOY_HOST         swarm manager hostname or IP (required unless DOCKER_HOST is set)
#   DEPLOY_USER         SSH user there that can run docker (default: root)
#   DEPLOY_SSH_KEY      private key to log in with (default: your SSH agent/config)
#   DEPLOY_KNOWN_HOSTS  known_hosts line(s) for DEPLOY_HOST (default: trust on first use)
#   IMAGE_TAG           image tag to deploy (default: latest). Use a unique tag per
#                       deploy, e.g. a commit SHA — config files are versioned by it.
#   DOCKER_HOST         set instead of DEPLOY_HOST to target any Docker daemon directly
#
# Also needed: a .env file here with the application's runtime variables
# (NAME=value lines, values unquoted), and docker logged in to
# {{REGISTRY_HOST}}, e.g.:
#   echo "$GHCR_TOKEN" | docker login {{REGISTRY_HOST}} -u {{REGISTRY_USERNAME}} --password-stdin
#
# Usage:
#   DEPLOY_HOST=203.0.113.10 IMAGE_TAG=abc1234 ./deploy.sh

set -euo pipefail
cd "$(dirname "$0")"

export IMAGE_TAG=${IMAGE_TAG:-latest}

if [ -z "${DOCKER_HOST:-}" ]; then
	: "${DEPLOY_HOST:?set DEPLOY_HOST (or DOCKER_HOST)}"
	if [ -n "${DEPLOY_SSH_KEY:-}" ]; then
		eval "$(ssh-agent -s)" >/dev/null
		trap 'ssh-agent -k >/dev/null' EXIT
		printf '%s\n' "$DEPLOY_SSH_KEY" | ssh-add - >/dev/null 2>&1
	fi
	mkdir -p ~/.ssh && chmod 700 ~/.ssh
	if [ -n "${DEPLOY_KNOWN_HOSTS:-}" ]; then
		printf '%s\n' "$DEPLOY_KNOWN_HOSTS" >>~/.ssh/known_hosts
	elif ! ssh-keygen -F "$DEPLOY_HOST" >/dev/null 2>&1; then
		echo "warning: trusting $DEPLOY_HOST's SSH host key on first use; set DEPLOY_KNOWN_HOSTS to pin it" >&2
		ssh-keyscan -H "$DEPLOY_HOST" >>~/.ssh/known_hosts 2>/dev/null
	fi
	export DOCKER_HOST="ssh://${DEPLOY_USER:-root}@$DEPLOY_HOST"
fi

# Export .env for ${VAR} interpolation in the compose files: unlike
# `docker compose`, `docker stack deploy` doesn't read .env itself. Lines
# are taken literally (no shell evaluation), the same way --env-file does.
if [ -f .env ]; then
	while IFS= read -r line || [ -n "$line" ]; do
		case "$line" in '' | '#'*) continue ;; esac
		export "${line%%=*}=${line#*=}"
	done <.env
fi

docker swarm init >/dev/null 2>&1 || true

# Pull with retry — a registry can take a moment to serve a freshly
# pushed image.
# kiln:insert pull

# kiln:insert migrate

echo "=== Deploying stack {{STACK_NAME}} ($IMAGE_TAG) ==="
for i in 1 2 3; do
	docker stack deploy \
		--with-registry-auth \
		--prune \
		-c docker-compose.yml \
		-c docker-compose.swarm.yml \
		{{STACK_NAME}} && break
	if [ "$i" = 3 ]; then exit 1; fi
	echo "stack deploy attempt $i failed, retrying..."
	sleep 5
done

echo "=== Verifying rollout ==="
# kiln:insert verify

docker image prune -f >/dev/null
echo "=== Done ==="
