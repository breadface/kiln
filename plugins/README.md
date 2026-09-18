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

### Kind selection: auto-select vs. opt-in

`backend` and `hosting` are the only kinds `kiln new` auto-selects when
exactly one plugin exists for them — nearly every project wants one of each.
Every other kind (`frontend`, `mobile`, `payments`, `glue`) is **opt-in
only**, resolved solely via an explicit flag (e.g. `--mobile kmp-compose`)
or `--set`, even if it's the only plugin of that kind available. Don't add a
new kind to the auto-select list without a real reason — the point is that
`kiln new` never silently adds something a project didn't ask for.

### Registering a new plugin

A new plugin isn't discoverable until it's also added to the `//go:embed`
directive in `plugins/embed.go` (e.g. `//go:embed backend hosting mobile`)
— that directive lists top-level kind directories explicitly, not a
wildcard. Forgetting this is a common first mistake: the plugin builds fine,
`go:embed` just silently doesn't include it.

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
  in the generated project. **This is specific to Go source** — a `.kt`,
  `.swift`, `.gradle.kts`, or any other non-Go file never needs `.tmpl`,
  since none of it confuses the kiln repo's own Go tooling (confirmed
  directly: `mobile/kmp-compose`'s files/ tree has zero `.tmpl` files).
- **Dot-prefixed files are excluded from `go:embed` by default** (e.g.
  `.env.example`). Prefix the segment with **`dot__`** in `files/` instead —
  `files/dot__env.example` is copied out as `.env.example`, and
  `files/dot__github/workflows/deploy.yml.tmpl` is copied out as
  `.github/workflows/deploy.yml`.

Both escapes are stripped per path segment, after placeholder rendering, by
the same mechanism that renders `{{APP_NAME}}` in paths — a plugin author
doesn't need to do anything beyond naming the file correctly in `files/`.
Binary files (a real `gradle-wrapper.jar`, an image) can be committed and
embedded directly — placeholder substitution is a regex over raw bytes, and
`{{[A-Z0-9_]+}}` essentially never occurs by chance in real binary content;
`mobile/kmp-compose` ships a real Gradle wrapper jar this way, verified
byte-identical after the generate round trip in its e2e test.

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

### The base-file-plus-fragment pattern, generically

The mechanism above isn't specific to Caddy or Compose — it's the general
way Kiln lets independent plugins extend a shared file without knowing
about each other, and it's the pattern to reach for whenever a new plugin
needs to contribute into something another plugin owns:

1. One plugin ships the shared file under `files/` with a `# kiln:insert`
   marker line where contributions go (see `hosting/docker-swarm-hetzner`'s
   `files/Caddyfile`). That plugin is the base's owner — its own
   contribution goes through the exact same fragment mechanism as anyone
   else's, for symmetry (see `backend/go-grpc-gateway`'s `buf.gen.yaml`
   below, which ships only the marker itself and gets 100% of its content,
   including its own, from fragments).
2. Any plugin — including the owner — drops a `fragments/<Base>.fragment`
   file with just its own contribution. `MergeTextBlocks` inserts each
   fragment at the marker, in `Kind` order, each preceded by a small
   provenance comment.
3. If a target is optional rather than always-present (unlike Caddyfile,
   which always has an owner because hosting ships both the base and its
   fragment together), a fragment aimed at a base file nothing shipped is
   silently skipped, not an error — see `backend/go-grpc-gateway` +
   `mobile/kmp-compose` below.

**Worked example — `buf.gen.yaml` across languages:** `backend/go-grpc-gateway`
owns `files/buf.gen.yaml` (just a `version`/`managed` shell plus the
marker) and contributes its own Go/grpc/grpc-gateway plugin entries via
`fragments/buf.gen.yaml.fragment`. `mobile/kmp-compose` contributes a
second, independent `fragments/buf.gen.yaml.fragment` with Kotlin/Java
entries targeting the exact same base file — neither plugin imports or
references the other. Picking both merges both languages' entries into one
`buf.gen.yaml`; picking `kmp-compose` without `go-grpc-gateway` means no
plugin ever ships the base file, so `kmp-compose`'s fragment is skipped and
no `buf.gen.yaml` is generated at all. Adding a third language's codegen
later (e.g. a TypeScript frontend) means shipping one more
`buf.gen.yaml.fragment` — no change to either existing plugin.

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
