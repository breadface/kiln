# Plugin authoring format

A Kiln plugin is a directory containing:

```
<kind>/<name>/
  plugin.yaml     # manifest — see below
  files/          # copied into the generated project, after rendering
  fragments/      # contributions to shared files other plugins own (see "Fragments")
  bindings/       # fragments rendered once per contract provider (see "Contracts")
```

## `plugin.yaml`

```yaml
name: go-api
kind: backend          # backend | frontend | mobile | payments | edge | registry | runtime | ci | glue
version: 0.2.0
description: Minimal Go HTTP API starter (net/http, Postgres+gorm, env config)

requires: []            # "kind" or "kind/name" refs this plugin needs selected
conflicts: []            # "kind" or "kind/name" refs incompatible with this plugin

placeholders:
  - name: MODULE_PATH
    description: Go module path for the generated API
    required: true
    flag: module               # exposed as `kiln new --module`
  - name: SERVICE_NAME
    description: Service name
    default: "{{APP_NAME}}-api" # defaults may reference other placeholders
  - name: GHCR_OWNER
    required: true
    derive:                    # computed from another value when not set
      from: MODULE_PATH
      match: '^github\.com/([^/]+)/'
      template: "$1"

provides:                      # contracts this plugin implements — see "Contracts"
  - contract: http-service@2
    with:
      SERVICE_NAME: "{{SERVICE_NAME}}"
      PORT: "{{PORT}}"
consumes: []                   # contracts this plugin works with — see "Contracts"

next_steps:                    # printed after generation, rendered
  - cd {{APP_NAME}}-api && make run

post_copy: hooks/post_copy.sh   # optional, run after all files are copied + merged
contributes_go_module: true      # this plugin ships its own go.mod at project root
executable:                       # glob patterns (relative to files/) to chmod +x after copy
  - "*.sh"
```

### Values

Kiln itself defines exactly one value, `APP_NAME` (the slugified project
name). Every other value is a placeholder some plugin declares, resolved in
this order:

1. an explicit value — a plugin-declared `flag:` (e.g. `--module`) or
   `--set KEY=VALUE`, which wins over the flag;
2. `derive:` — either `match` + `template` (a regexp over another value,
   expanded with its capture groups; no match leaves it unset) or `replace`
   (literal substring replacements, e.g. `{".": "/"}` turns
   `com.example.app` into `com/example/app`);
3. `default:` — which may reference other placeholders and resolves once
   they're known.

Anything `required` and still unset is prompted for (or reported, with
`--yes`). Two plugins can declare the same placeholder — they then share one
value — but that's for genuinely shared concepts; to use another plugin's
values, consume a contract instead (see below).

### Kind selection: auto-select vs. opt-in

`backend`, `edge`, `registry`, `runtime` and `ci` are the kinds `kiln new`
auto-selects when exactly one plugin exists for them — nearly every project
wants an app and a way to ship it.
Every other kind (`frontend`, `mobile`, `payments`, `glue`) is **opt-in
only**, resolved solely via an explicit flag (e.g. `--mobile kmp-compose`)
or `--set`, even if it's the only plugin of that kind available. Don't add a
new kind to the auto-select list without a real reason — the point is that
`kiln new` never silently adds something a project didn't ask for.

### Registering a new plugin

Nothing to do: `plugins/embed.go` embeds every `<kind>/<name>/` directory.
The kind must be one Kiln knows (see `plugin.Kinds()`).

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

## Contracts

A contract is the interface between plugins: a schema of named fields in
[`contracts/`](../contracts/README.md), versioned by major number (e.g.
`http-service@2`). Plugins never reference each other by name; one
**provides** a contract by filling in its fields, another **consumes** it
and is generated against whatever providers were selected.

```yaml
# backend/go-api
provides:
  - contract: http-service@2
    with: { SERVICE_NAME: "{{SERVICE_NAME}}", PORT: "{{PORT}}" }
  - contract: oci-image@1
    with: { SERVICE_NAME: "{{SERVICE_NAME}}", BUILD_CONTEXT: "{{APP_NAME}}-api" }
  - contract: env@1
    with: { ENV_NAME: DATABASE_URL }

# edge/caddy
consumes:
  - contract: http-service@2
    min: 1              # error unless at least one selected plugin provides it

# ci/github-actions
consumes:
  - contract: env@1     # min defaults to 0, max to unlimited

# mobile/kmp-compose
consumes:
  - contract: http-service@2
    min: 1
    max: 1
    as: API             # exports the provider's fields as {{API_PORT}}, {{API_SERVICE_NAME}}, …
```

Every `provides` entry is validated against its schema — required fields
present, no unknown fields, values fully resolved — whether or not anything
consumes it. A plugin can provide the same contract several times (one
`env@1` per variable).

A consumer uses its bindings in two ways:

- **`bindings/<contract-name>/`** holds fragments (same naming as
  `fragments/`, below) rendered **once per bound provider**, with that
  provider's contract fields layered over the global values. This is how
  infrastructure plugins build, deploy and route to N services without any
  template logic: `edge/caddy`'s `bindings/http-service/Caddyfile@routes.fragment`
  is rendered for each service. A plugin that adds a service or a secret —
  say, a payments provider — just provides `http-service@2` /
  `oci-image@1` / `env@1`, and the edge, CI and runtime pick it up with no
  change (see `internal/cli/contracts_test.go`).
- **`as:`** (requires `max: 1`) exports the single provider's fields as
  global placeholders, for use anywhere in the consumer's `files/`.

## Fragments

Some files are contributed to by more than one plugin at once (e.g. a
backend plugin's API service and the edge plugin's Caddy service both
belong in the same `docker-compose.yml`). Plugins don't own these files
directly; they contribute a fragment instead:

- `fragments/<path>.fragment.yml` (or `.yaml`) — a YAML document
  deep-merged into `<path>.yml`, e.g. `docker-compose.fragment.yml` into
  `docker-compose.yml`. It's fine for two plugins to declare the exact
  same key with the same value; it's an error for them to declare the same
  key with different values. Use this for YAML that several contracts
  write into the same keys (`runtime/docker-swarm`'s
  `docker-compose.swarm.yml`); comments in a YAML base don't survive the
  merge.
- `fragments/<path>.fragment` — plain text, inserted into the base file at
  `<path>` (relative to the project root) at its unnamed marker line.
  `<path>` goes through the same rendering and filename escapes as `files/`,
  so `fragments/dot__github/workflows/deploy.yml@build.fragment` targets
  `.github/workflows/deploy.yml`.
- `fragments/<path>@<slot>.fragment` — the same, at the marker for a named
  slot.

A marker is a comment (`#`, `//`, `--`, `;`, or `<!-- … -->` for
Markdown) reading `kiln:insert` with an optional slot name:

```yaml
jobs:
  build:
    steps:
      - uses: actions/checkout@v4
      # kiln:insert build
```

Each inserted fragment is prefixed with the marker's indentation — write
fragments unindented — and preceded by a provenance comment in the
marker's own comment syntax. Markers are removed from the output once
every fragment is merged, so a slot nobody contributed to just disappears.

Fragments are applied in `Kind` order (backend → frontend → mobile →
payments → edge → registry → runtime → ci → glue), then binding fragments
in the same order, so
generated files read top-to-bottom as "application services, then
infrastructure."

### The base-file-plus-fragment pattern, generically

The mechanism above isn't specific to Caddy or Compose — it's the general
way Kiln lets independent plugins extend a shared file without knowing
about each other, and it's the pattern to reach for whenever a new plugin
needs to contribute into something another plugin owns:

1. One plugin ships the shared file under `files/` with marker lines where
   contributions go (see `ci/github-actions`'
   `files/dot__github/workflows/deploy.yml`, which has `build`,
   `env-vars`, `env-lines` and `secrets` slots). That plugin is the base's owner — its own
   contribution goes through the exact same fragment mechanism as anyone
   else's, for symmetry (see `backend/go-grpc-gateway`'s `buf.gen.yaml`
   below, which ships only the marker itself and gets 100% of its content,
   including its own, from fragments).
2. Any plugin — including the owner — drops a fragment file with just its
   own contribution.
3. A fragment aimed at a base file nothing shipped is silently skipped, not
   an error — the target is optional (see `backend/go-grpc-gateway` +
   `mobile/kmp-compose` below).

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

Use fragments when the base file belongs to a *specific* plugin and you're
adding to it once. Use contracts when a plugin needs to repeat something
for *every* provider of a concept, whoever they are.

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
