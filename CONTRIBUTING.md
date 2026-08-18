# Contributing to Kiln

## Building and testing

```sh
go build ./...
go vet ./...
go test ./...
```

`go test ./...` includes an end-to-end test (`internal/cli/new_e2e_test.go`)
that runs the real `kiln new` command against the real embedded plugins,
then builds the generated Go module — it shells out over the network (`go
mod tidy`). Skip it with `go test -short ./...` if you're offline.

To try generation by hand:

```sh
go run ./cmd/kiln new demoapp \
  --module github.com/you/demoapp-api \
  --domain demo.example.com \
  --admin-email you@example.com \
  --dir /tmp/kiln-try --yes
```

## Adding or changing a plugin

See [plugins/README.md](plugins/README.md) for the manifest format, the
`files/`/`fragments/` split, and the filename escapes required because
plugin content is embedded via `//go:embed` (`.tmpl` suffix, `dot__` prefix
— read that section before adding a `.go` file or a dotfile to a plugin, or
it will silently fail to embed).

After adding or changing plugin content, regenerate a project and check it
by hand — `docker compose config` (and, for the hosting plugin,
`docker compose -f docker-compose.yml -f docker-compose.swarm.yml config`)
catches Compose schema and merge mistakes that a Go test won't.

## Commit conventions

Commits are atomic and scoped to one logical change — prefer several small,
buildable commits over one large one. Messages follow a rough
`type(scope): summary` shape (`feat`, `fix`, `docs`, `test`, `chore`, `ci`),
matching the existing history (`git log --oneline`). Explain *why* in the
body when the commit fixes something non-obvious, not just what changed.

## Documentation

Update the relevant `README.md` (root or `plugins/`) in the same change as
the code it describes, not as a follow-up.
