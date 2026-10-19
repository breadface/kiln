# Kiln

Kiln is a CLI that scaffolds new projects by composing independent, reusable
plugins — a backend, a hosting setup, a payment provider, and so on — instead
of copy-pasting boilerplate between projects and letting it drift.

```sh
kiln new myapp \
  --backend go-api \
  --module github.com/you/myapp-api \
  --domain myapp.example.com \
  --admin-email you@example.com
```

That generates a working project: a minimal Go HTTP API (stdlib routing,
Postgres via gorm, env-based config) and a self-hosted deploy setup (GitHub
Actions → GHCR → Docker Swarm behind Caddy, with automatic TLS). `--backend`
is required once more than one backend plugin exists (see Status below) —
`go-api` is the plain REST option; `go-grpc-gateway` scaffolds the same app
proto-first instead, with buf generating Go stubs a real `grpc.Server`
implements, fronted by a grpc-gateway REST proxy.

```
myapp/
├── myapp-api/              # the Go API — go build ./... works standalone
├── docker-compose.yml      # local dev: docker compose up
├── docker-compose.swarm.yml # production overlay
├── Caddyfile
├── deploy.sh                # manual deploy escape hatch
├── .github/workflows/deploy.yml
└── kiln.yaml                 # records what generated this project
```

Add a mobile client (opt-in — `mobile` is never auto-selected, see
[plugins/README.md](plugins/README.md)) with `--mobile`:

```sh
kiln new myapp \
  --backend go-api \
  --module github.com/you/myapp-api \
  --domain myapp.example.com \
  --admin-email you@example.com \
  --mobile kmp-compose \
  --mobile-package com.example.myapp
```

That adds `myapp-mobile/`: a Kotlin Multiplatform + Compose Multiplatform
client (a single `composeApp` module plus a thin `iosApp` Xcode wrapper) that
calls the generated API's real `GET /api/hello` over plain Ktor REST — no
gRPC, no auth, nothing fake.

## Install

```sh
go install github.com/breadface/kiln/cmd/kiln@latest
```

## Status

Early. Kiln currently ships four plugins: `backend/go-api`,
`backend/go-grpc-gateway`, `hosting/docker-swarm-hetzner`, and
`mobile/kmp-compose`, composed together by `kiln new`. More backend
languages, frontends, and payment providers are additive — see
[plugins/README.md](plugins/README.md) for the plugin format and
[CONTRIBUTING.md](CONTRIBUTING.md) for how to add one.

## How it works

Each plugin declares a `plugin.yaml` manifest and ships a `files/` tree
that's copied into the generated project (with `{{PLACEHOLDER}}` values
substituted). Plugins never refer to each other: they meet at versioned
**contracts** ([contracts/](contracts/README.md)) — a backend *provides*
`http-service@1` and `env@1`, a hosting plugin *consumes* them and builds,
deploys, routes and wires secrets for every provider, whichever plugins
those are. Shared files like `docker-compose.yml`, `Caddyfile` or a CI
workflow are extended through fragments inserted at named
`# kiln:insert <slot>` markers. See [plugins/README.md](plugins/README.md)
for the full format, and [RFC 0001](docs/rfcs/0001-composable-architecture.md)
for where this is going.

Plugins currently ship bundled inside the `kiln` binary itself. The design
leaves room for plugins to move to independently-hosted repositories later
without changing the manifest format or how a generated project looks.

## License

[MIT](LICENSE)
