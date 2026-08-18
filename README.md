# Kiln

Kiln is a CLI that scaffolds new projects by composing independent, reusable
plugins — a backend, a hosting setup, a payment provider, and so on — instead
of copy-pasting boilerplate between projects and letting it drift.

```sh
kiln new myapp \
  --module github.com/you/myapp-api \
  --domain myapp.example.com \
  --admin-email you@example.com
```

That generates a working project: a minimal Go HTTP API (stdlib routing,
Postgres via gorm, env-based config) and a self-hosted deploy setup (GitHub
Actions → GHCR → Docker Swarm behind Caddy, with automatic TLS).

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

## Install

```sh
go install github.com/surebits/kiln/cmd/kiln@latest
```

## Status

Early. Kiln currently ships two plugins: `backend/go-api` and
`hosting/docker-swarm-hetzner`, composed together by `kiln new`. More
languages, frontends, mobile targets, and payment providers are additive —
see [plugins/README.md](plugins/README.md) for the plugin format and
[CONTRIBUTING.md](CONTRIBUTING.md) for how to add one.

## How it works

Each plugin declares a `plugin.yaml` manifest and ships a `files/` tree
that's copied into the generated project (with `{{PLACEHOLDER}}` values
substituted) plus an optional `fragments/` contribution to shared root files
like `docker-compose.yml` and `Caddyfile` — since more than one plugin needs
to add to those without knowing about each other. See
[plugins/README.md](plugins/README.md) for the full format.

Plugins currently ship bundled inside the `kiln` binary itself. The design
leaves room for plugins to move to independently-hosted repositories later
without changing the manifest format or how a generated project looks.

## License

[MIT](LICENSE)
