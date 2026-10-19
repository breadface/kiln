# RFC 0001: Kiln as a composer of independent plugins

- Status: accepted; milestones 1–3 implemented
- Date: 2026-10-19

## Goal

Kiln generates a working, production-shaped project from one pick per
concern — language, framework, payments, auth, cloud, CI, repo layout —
with no lock-in to any provider, and keeps that project upgradeable as its
plugins improve. Every choice is machine-readable, so an agent (via an MCP
server) can compose stacks as reliably as a person.

The value over writing a scaffold by hand, or asking an AI for one, is
**verification and maintenance**: a plugin combination Kiln offers has been
generated, built and booted in CI, and a project generated today can take
tomorrow's plugin fixes.

## Principles

1. **The core composes; it knows nothing.** No language, cloud, container
   runtime or provider appears in `internal/`. Anything plugin-specific is
   declared in a manifest.
2. **Plugins meet at contracts, never at names.** A plugin may not assume
   another plugin exists, or what its placeholders are called. It provides
   or consumes a versioned contract.
3. **Generation is a pure function.** Selection + values → files, in
   memory, with no I/O (runs in WebAssembly too). Side effects (writing,
   hooks) happen after.
4. **Every generated project works on day one, offline.** Capabilities that
   need an external account ship a local implementation.
5. **Every combination Kiln allows is tested.** If it can't be tested, it
   shouldn't be selectable.

## Milestones

### 1. Contracts, and a core with no plugin knowledge — done

- `contracts/` schemas (`http-service@1`, `migration@1`, `env@1`);
  `provides` / `consumes` (with `min`, `max`, `as`) in manifests; bindings
  resolved and validated in `generate.ResolveBindings`, recorded in
  `kiln.yaml`.
- Per-binding fragments (`bindings/<contract>/`), named marker slots,
  marker-relative indentation, fragments at any path.
- Values: only `APP_NAME` is built in; `flag:`, `derive:`, defaults that
  reference other values, and `next_steps:` replace the hardcoded
  `--module`/`--domain`/`GHCR_OWNER`/`MOBILE_PACKAGE_PATH`/`SERVICE_NAME`
  logic. CLI flags are generated from manifests.
- The hosting plugin no longer references any backend value; a new service
  or secret from any plugin is built, deployed, routed and wired with no
  change to it.

Known leftovers: `ContributesGoModule` / `go.work` and the special-cased
`docker-compose.fragment.yml` are still in the engine; `Kind` is still a
closed list; selection is still one plugin per kind.

### 2. Plan / apply and a lockfile — done

- `kiln plan` resolves selection, values and bindings and renders every
  file in memory, with no side effects; `--json` for tools, `--out` to
  save. `kiln apply` writes a saved plan or any project's `kiln.yaml`;
  `kiln new` is plan + apply.
- `kiln.yaml` is the lockfile: plugin versions and content digests, values,
  bindings, and a hash of every rendered file. `apply` refuses to run if a
  plugin's version or digest differs from what was recorded.
- Golden snapshots per supported combination
  (`internal/cli/testdata/plans/`) make every plugin change reviewable as a
  diff of generated output; a reproducibility test checks every
  combination regenerates byte for byte from its own `kiln.yaml`.

Finding (resolved for the bundled backends in milestone 3): `post_copy`
hooks break reproducibility. `go mod tidy` rewrites
`go.mod`/`go.sum` after rendering, with results that depend on the network
at that moment, so `kiln.yaml`'s hashes describe Kiln's rendered output,
not what's on disk. Before `kiln upgrade` (milestone 6) can treat "differs
from recorded hash" as "edited by the user", hooks must either become
declarative steps whose output Kiln records, or have their effects hashed
after they run.

### 3. Split hosting into independent axes — done

`hosting/docker-swarm-hetzner` became four kinds, each replaceable on its
own, meeting only at contracts:

| Kind | Plugin | Provides | Consumes |
|---|---|---|---|
| edge | `caddy` | `config-file@1` | `http-service@2` |
| registry | `ghcr` | `image-registry@1` | — |
| runtime | `docker-swarm` | `deploy-target@1`, `deploy-secret@1` | `oci-image@1`, `migration@1`, `config-file@1`, `image-registry@1` |
| ci | `github-actions` | — | `oci-image@1`, `env@1`, `deploy-secret@1`, `deploy-target@1`, `image-registry@1` |

`http-service@2` drops the build fields, which move to `oci-image@1`.
Structured YAML fragments work for any path, and Markdown files can take
`<!-- kiln:insert -->` slots.

Splitting it surfaced, and fixed, two bugs in the old pipeline: the deploy
ran on the server against files nothing ever copied there, and `docker
stack deploy` never read `.env`, so the application's variables reached
containers empty. `deploy.sh` now runs on the CI runner or a laptop and
drives the server's Docker over SSH using the repository's files, and
exports `.env` itself. A third: the backends resolved dependencies with
`go mod tidy` at generation time, which had drifted past their
Dockerfiles' Go versions, so neither generated image built. Backends now
pin every dependency and ship `go.sum`, and have no hooks — generated
projects are fully deterministic, and `kiln.yaml`'s hashes match the disk.

`scripts/smoke/docker-swarm.sh` deploys a generated project for real, over
SSH, to a disposable Docker-in-Docker swarm, and checks it — the seed of
milestone 5's conformance kit.

Deferred: a `packaging` kind (the backend ships its own Dockerfile, so
there's no language-agnostic packager to split out until buildpacks or
Nix arrive), `layout` (monorepo vs polyrepo), `secrets` (CI secrets are the
only store today), and allowing several plugins of one kind. Old Swarm
configs accumulate on the server, one per deploy.

### 4. Capabilities as runtime ports

For each capability (payments, auth, email, storage, queue), the backend
exposes a language-native interface and an extension slot (e.g. a generated
`wiring.go` with a `// kiln:insert` marker); capability plugins provide
adapters per language through a contract, and every capability ships a
local/fake adapter. Payments first: a fake provider, then Stripe.

### 5. Conformance kit

`kiln plugin test` generates every combination the contracts allow, builds
it, boots it, and hits each `HEALTH_PATH`. CI derives its matrix from the
contracts, so contributors can't break combinations they never tried.

### 6. Lifecycle: `kiln add`, `kiln upgrade`

Driven by `kiln.yaml`: three-way merge of old pristine output, new pristine
output and the user's working tree; Kiln-owned vs user-owned files; `eject`.

### 7. External plugins and an MCP server

A git-backed `plugin.Loader` with a signed index; third-party `post_copy`
hooks replaced by declarative steps or sandboxed WASM. An MCP server
exposing list/describe/plan/apply/add over the same `generate` package the
CLI uses.
