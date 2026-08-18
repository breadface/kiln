# Plugin authoring format

A Kiln plugin is a directory containing:

```
<kind>/<name>/
  plugin.yaml     # manifest — see below
  files/          # copied into the generated project, after rendering
  fragments/      # contributions to shared root files (see "Fragments")
```

## `plugin.yaml`

```yaml
name: go-api
kind: backend          # backend | frontend | mobile | payments | hosting | glue
version: 0.1.0
description: Minimal Go HTTP API starter (net/http, Postgres+gorm, env config)

requires: []            # "kind" or "kind/name" refs this plugin needs selected
conflicts: []            # "kind" or "kind/name" refs incompatible with this plugin

placeholders:
  - name: MODULE_PATH
    description: Go module path for the generated API
    required: true
  - name: PORT
    description: HTTP port the API listens on
    default: "8080"

post_copy: hooks/post_copy.sh   # optional, run after all files are copied + merged
contributes_go_module: true      # this plugin ships its own go.mod at project root
executable:                       # glob patterns (relative to files/) to chmod +x after copy
  - "*.sh"
```

## Files

Everything under `files/` is copied into the generated project. Both file
contents and path segments are rendered: a placeholder named `APP_NAME` turns
`files/{{APP_NAME}}-api/main.go` into `demoapp-api/main.go` when generating a
project called `demoapp`.

Placeholders use bare `{{NAME}}` syntax (all-caps letters, digits, and
underscores). This is deliberately not Go's `text/template` — no dot-prefixed
field access, no logic — so it can't collide with `${VAR}` / `${VAR:-default}`
shell/Compose interpolation syntax already present in files like
`docker-compose.yml`. An unresolved placeholder is left in the output as-is,
which generation tooling treats as a failure signal.

### Filename escapes

Plugin content lives inside the `kiln` repo's own Go module and is embedded
into the binary via `//go:embed`. Both of those facts mean some filenames a
generated project needs can't appear literally under `files/`:

- **Any `*.go` file, and `go.mod`/`go.sum`, must end in an extra `.tmpl`** —
  e.g. `files/{{APP_NAME}}-api/main.go.tmpl`, `files/{{APP_NAME}}-api/go.mod.tmpl`.
  Two independent reasons force this: `go build ./...`/`go test ./...` run
  from the kiln repo root would otherwise try to compile these as real
  packages of the kiln module (and fail — they're not, and their imports
  reference an unresolved `{{MODULE_PATH}}`); and `go:embed` treats any
  directory containing its own `go.mod` as a separate module boundary and
  silently drops everything under it. Naming them `*.tmpl` sidesteps both.
  The `.tmpl` suffix is stripped on copy — `main.go.tmpl` becomes `main.go`
  in the generated project.
- **Dot-prefixed files are excluded from `go:embed` by default** (e.g.
  `.env.example`). Prefix the segment with **`dot__`** in `files/` instead —
  `files/dot__env.example` is copied out as `.env.example`, and
  `files/dot__github/workflows/deploy.yml.tmpl` is copied out as
  `.github/workflows/deploy.yml`.

Both escapes are stripped per path segment, after placeholder rendering, by
the same mechanism that renders `{{APP_NAME}}` in paths — a plugin author
doesn't need to do anything beyond naming the file correctly in `files/`.

## Fragments

Some files — `docker-compose.yml`, `Caddyfile`, `go.work` — are contributed to
by more than one plugin at once (e.g. a backend plugin's API service and a
hosting plugin's Caddy service both belong in the same `docker-compose.yml`).
Plugins don't own these files directly; they contribute a fragment instead:

- `fragments/docker-compose.fragment.yml` — a YAML document merged into the
  final `docker-compose.yml`. It's fine for two plugins to declare the exact
  same key with the same value; it's an error for them to declare the same
  key with different values.
- `fragments/Caddyfile.fragment`, `fragments/go.work.fragment` — plain text,
  concatenated into the base file (owned by whichever plugin ships it as
  `files/Caddyfile` etc.) at a `# kiln:insert` marker line.

Fragments from multiple plugins are applied in `Kind` order (backend →
frontend → mobile → payments → hosting → glue), not alphabetically, so
generated files read top-to-bottom as "application services, then
infrastructure."

## Hooks

`post_copy` scripts run via POSIX `sh` after every selected plugin's files
are copied and all fragments are merged, with each resolved placeholder value
exported as a `KILN_<NAME>` environment variable. This is a v0 constraint —
plugin authors should assume a POSIX shell is available, not `cmd.exe`.

## Scope note

Plugins currently ship bundled inside the `kiln` binary itself (see
`plugins/embed.go`) rather than as independently-hosted repositories. The
manifest format above is designed so this format doesn't need to change when
external plugin repositories are added later.
