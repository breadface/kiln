#!/usr/bin/env bash
# Real deploy smoke test for runtime/docker-swarm (with edge/caddy,
# registry/ghcr, ci/github-actions' contracts). Needs only Docker.
#
# Generates a project with `kiln new`, then deploys it with the generated
# deploy.sh — over real SSH, from a "runner" container, to a disposable
# Docker-in-Docker "server" — twice, and checks: services converge, the
# API answers through the overlay network, Caddy serves the domain, .env
# values reach the container verbatim, and a changed Caddyfile rolls out
# as a new Swarm config.
#
# The one deviation from production: images go to a registry inside the
# server instead of ghcr.io, by rewriting the image host in the generated
# files.
#
# With --payments, the project also gets payments/stripe, deployed with
# PAYMENTS_PROVIDER=fake, and the payment flow is run against the real
# stack and Postgres: create, pay, replay the outcome, read it back.
#
# Usage (from the repository root):
#   scripts/smoke/docker-swarm.sh [--backend go-api|go-grpc-gateway] [--payments]
set -euo pipefail

backend=go-api
payments=""
while [ $# -gt 0 ]; do
	case "$1" in
	--backend) backend=$2 && shift 2 ;;
	--payments) payments=1 && shift ;;
	*) echo "unknown argument: $1" >&2 && exit 2 ;;
	esac
done

work=$(mktemp -d)
net=kiln-smoke-$$
server=kiln-smoke-server-$$
runner=kiln-smoke-runner-$$

cleanup() {
	docker rm -f "$server" "$runner" >/dev/null 2>&1 || true
	docker network rm "$net" >/dev/null 2>&1 || true
	rm -rf "$work"
}
trap cleanup EXIT

step() { printf '\n=== %s ===\n' "$*"; }
fail() {
	echo "FAIL: $*" >&2
	exit 1
}
on_server() { docker exec "$server" docker "$@"; }
curl_overlay() { on_server run --rm --network demo_default curlimages/curl:8.10.1 -s "$@"; }

step "Generating project ($backend${payments:+, payments/stripe})"
go run ./cmd/kiln new demo --backend "$backend" ${payments:+--payments stripe} \
	--module github.com/example/demo-api \
	--domain demo.example.com --admin-email admin@example.com \
	--dir "$work" --yes >/dev/null
project=$work/demo
sed -i.bak 's#ghcr.io/example#localhost:5000/example#g' "$project/deploy.sh" "$project/docker-compose.swarm.yml"
rm -f "$project"/*.bak

step "Starting server (dind + sshd) and runner"
docker network create "$net" >/dev/null
docker run -d --privileged --name "$server" --network "$net" -e DOCKER_TLS_CERTDIR= docker:27-dind >/dev/null
for _ in $(seq 1 60); do on_server info >/dev/null 2>&1 && break; sleep 1; done
on_server info >/dev/null 2>&1 || fail "server's docker daemon never came up"
ssh-keygen -q -t ed25519 -N '' -f "$work/key"
docker exec "$server" sh -c 'apk add -q --no-cache --upgrade openssh >/dev/null && ssh-keygen -A >/dev/null &&
	mkdir -p /root/.ssh && sed -i "s/^#*PermitRootLogin.*/PermitRootLogin prohibit-password/" /etc/ssh/sshd_config && /usr/sbin/sshd'
docker cp "$work/key.pub" "$server:/root/.ssh/authorized_keys"
docker exec "$server" sh -c 'chown root:root /root/.ssh/authorized_keys && chmod 600 /root/.ssh/authorized_keys'
on_server run -d --name registry -p 5000:5000 registry:2 >/dev/null
on_server run -d --name pg -p 5432:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=demo postgres:16-alpine >/dev/null
server_ip=$(docker inspect -f "{{(index .NetworkSettings.Networks \"$net\").IPAddress}}" "$server")
docker run -d --name "$runner" --network "$net" -v "$project:/project" -w /project docker:27-cli sleep infinity >/dev/null
docker exec "$runner" apk add -q --no-cache --upgrade bash openssh-client >/dev/null
docker cp "$work/key" "$runner:/root/deploy_key"

# Values with shell metacharacters, to prove .env is passed verbatim.
# shellcheck disable=SC2016 # a literal $, on purpose
jwt_secret='s3cr&t value $HOME'
printf '%s\n' \
	"DATABASE_URL=postgres://postgres:postgres@$server_ip:5432/demo?sslmode=disable" \
	"JWT_SECRET=$jwt_secret" \
	"CORS_ORIGINS=*" >"$project/.env"
if [ -n "$payments" ]; then echo "PAYMENTS_PROVIDER=fake" >>"$project/.env"; fi

deploy() {
	local tag=$1
	step "Building and pushing image $tag"
	docker exec -e DOCKER_HOST="tcp://$server:2375" "$runner" sh -c \
		"docker build -q -t localhost:5000/example/demo-api:$tag ./demo-api >/dev/null && docker push -q localhost:5000/example/demo-api:$tag >/dev/null"
	step "Deploying $tag with the generated deploy.sh over SSH"
	docker exec -e DEPLOY_HOST="$server" -e IMAGE_TAG="$tag" "$runner" \
		bash -c 'DEPLOY_SSH_KEY="$(cat /root/deploy_key)" ./deploy.sh'
}

deploy smoke1

step "Checking the running stack"
health=$(curl_overlay http://demo-api:8080/api/health)
[ "$health" = '{"status":"ok"}' ] || fail "health check returned: $health"
echo "api health: $health"
code=$(curl_overlay -o /dev/null -w '%{http_code}' -H 'Host: demo.example.com' http://caddy/api/health)
[ "$code" = 308 ] || fail "caddy returned $code for demo.example.com, want 308 (redirect to https)"
echo "caddy serves demo.example.com: $code"
container=$(on_server ps -q -f name=demo_demo-api | head -1)
got_secret=$(on_server exec "$container" printenv JWT_SECRET)
[ "$got_secret" = "$jwt_secret" ] || fail "JWT_SECRET reached the container as [$got_secret], want [$jwt_secret]"
echo "env passed verbatim: JWT_SECRET=[$got_secret]"

if [ -n "$payments" ]; then
	step "Running the payment flow (fake provider, real Postgres)"
	created=$(curl_overlay -X POST -H 'Content-Type: application/json' \
		-d '{"amount":1500,"currency":"usd","description":"Smoke","success_url":"https://demo.example.com/ok","cancel_url":"https://demo.example.com/no"}' \
		http://demo-api:8080/api/payments)
	id=$(printf '%s' "$created" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
	checkout=$(printf '%s' "$created" | sed -n 's/.*"checkout_url":"\([^"]*\)".*/\1/p')
	if [ -z "$id" ] || [ -z "$checkout" ]; then fail "creating a payment returned: $created"; fi
	echo "created $id"
	pay() { curl_overlay -o /dev/null -w '%{http_code} %{redirect_url}' -d "result=$1" "http://demo-api:8080$checkout"; }
	paid=$(pay succeeded)
	[ "$paid" = "303 https://demo.example.com/ok" ] || fail "paying returned: $paid"
	pay failed >/dev/null # a later outcome must not change a final status
	status=$(curl_overlay "http://demo-api:8080/api/payments/$id" | sed -n 's/.*"status":"\([^"]*\)".*/\1/p')
	[ "$status" = succeeded ] || fail "payment status is $status, want succeeded"
	echo "paid, redirected, replay ignored: $id is $status"
fi

printf '\n# smoke2\n' >>"$project/Caddyfile"
deploy smoke2

step "Checking the config rollover"
config=$(on_server service inspect demo_caddy --format '{{range .Spec.TaskTemplate.ContainerSpec.Configs}}{{.ConfigName}}{{end}}')
[ "$config" = demo_caddyfile_smoke2 ] || fail "caddy uses config $config, want demo_caddyfile_smoke2"
echo "caddy rolled to $config"

step "PASS"
