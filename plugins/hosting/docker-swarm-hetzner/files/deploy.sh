#!/bin/bash
# Manual escape hatch — use this on the server if you need to deploy
# without going through GitHub Actions (e.g. a hotfix or CI is down).
#
# Requires: GHCR_TOKEN env var, IMAGE_TAG env var (or defaults to latest)
#
# Usage:
#   IMAGE_TAG=abc1234 GHCR_TOKEN=ghp_xxx ./deploy.sh

set -euo pipefail

IMAGE_TAG=${IMAGE_TAG:-latest}
GITHUB_ACTOR=${GITHUB_ACTOR:-{{GHCR_OWNER}}}

echo "=== Manual deploy: $IMAGE_TAG ==="

echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GITHUB_ACTOR" --password-stdin

# Create versioned Caddyfile config (skip if already exists — same SHA = same content)
docker config inspect "caddyfile_$IMAGE_TAG" >/dev/null 2>&1 || \
	docker config create "caddyfile_$IMAGE_TAG" ./Caddyfile

# kiln:insert migrate

echo "=== Deploying stack ==="
IMAGE_TAG=$IMAGE_TAG docker stack deploy \
	--with-registry-auth \
	--prune \
	-c docker-compose.yml \
	-c docker-compose.swarm.yml \
	{{STACK_NAME}}

docker image prune -f
echo "=== Done ==="
