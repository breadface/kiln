# Kiln

Kiln is a CLI that scaffolds new projects by composing independent, reusable
plugins — a backend, an edge proxy, a runtime, a CI pipeline, a payment
provider, and so on — instead
of copy-pasting boilerplate between projects and letting it drift.

```sh
kiln new myapp \
  --backend go-api \
  --module github.com/you/myapp-api \
  --domain myapp.example.com \
  --admin-email you@example.com
```

That generates a working project: a minimal Go HTTP API (stdlib routing,
Postgres via gorm, env-based config) and a self-hosted deploy setup, each
part its own plugin — Caddy with automatic TLS at the edge, images in GHCR,
Docker Swarm on any server you can SSH into, and a GitHub Actions pipeline
that builds, pushes and deploys. `--backend`
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
├── deploy.sh                # deploys over SSH — run by CI, or by hand
├── .github/workflows/deploy.yml
├── .github/DEPLOY.md        # every secret the pipeline needs
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

### Payments

Add one-time payments with `--payments stripe`. The backend gets a
`payments` package — `POST /api/payments` to start a hosted checkout,
`GET /api/payments/{id}`, and a signature-verified webhook endpoint — and
the Stripe adapter plugs into it. Locally, `PAYMENTS_PROVIDER=fake` swaps
in a checkout page served by the app itself, so the whole flow works with
no Stripe account; production always uses Stripe.

### Plan, then apply

`kiln plan` takes the same flags as `kiln new` and shows what it would
generate — plugins, contract bindings, values, every file — without
writing anything. `--json` prints it for tools and agents; `--out` saves it.

```sh
kiln plan myapp --backend go-api --module github.com/you/myapp-api \
  --domain myapp.example.com --admin-email you@example.com --out plan.yaml
kiln apply plan.yaml
```

Every generated project's `kiln.yaml` is itself a plan: it records each
plugin's version and content digest, every value, every contract binding,
and a hash of every generated file. `kiln apply path/to/kiln.yaml`
regenerates the identical project, and refuses to if any plugin has changed
since.

## Install

```sh
go install github.com/breadface/kiln/cmd/kiln@latest
```

## Status

Early. Kiln currently ships these plugins, composed together by
`kiln new`:

| Kind | Plugins |
|---|---|
| backend | `go-api`, `go-grpc-gateway` |
| mobile | `kmp-compose` (opt-in) |
| payments | `stripe` (opt-in) |
| edge | `caddy` |
| registry | `ghcr` |
| runtime | `docker-swarm` |
| ci | `github-actions` |

Each infrastructure piece is replaceable on its own — another runtime, CI
system or registry implements the same contracts. More backend languages,
frontends, and payment providers are additive too — see
[plugins/README.md](plugins/README.md) for the plugin format and
[CONTRIBUTING.md](CONTRIBUTING.md) for how to add one.

## How it works

Each plugin declares a `plugin.yaml` manifest and ships a `files/` tree
that's copied into the generated project (with `{{PLACEHOLDER}}` values
substituted). Plugins never refer to each other: they meet at versioned
**contracts** ([contracts/](contracts/README.md)) — a backend *provides*
`http-service@2`, `oci-image@1` and `env@1`; the edge routes to every
service, CI builds and pushes every image and wires every secret, and the
runtime runs every image — whichever plugins provided them. Shared files like `docker-compose.yml`, `Caddyfile` or a CI
workflow are extended through fragments inserted at named
`# kiln:insert <slot>` markers. See [plugins/README.md](plugins/README.md)
for the full format, and [RFC 0001](docs/rfcs/0001-composable-architecture.md)
for where this is going.

Plugins currently ship bundled inside the `kiln` binary itself. The design
leaves room for plugins to move to independently-hosted repositories later
without changing the manifest format or how a generated project looks.

## License

[MIT](LICENSE)
