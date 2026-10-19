# RFC 0001: Kiln as a composer of independent plugins

- Status: accepted; milestone 1 implemented
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

### 2. Plan / apply and a lockfile

`kiln plan` resolves selection, values and bindings into a lockfile (exact
plugin versions and content hashes) plus a file tree, with no side effects.
`kiln apply` writes it. Golden-snapshot tests over plans make every plugin
change reviewable as a diff of generated output.

### 3. Split hosting into independent axes

Today's `hosting/docker-swarm-hetzner` bundles CI, registry, packaging,
runtime, edge proxy and machine provider. Each becomes its own kind,
meeting at contracts: `packaging` (OCI image, buildpack, Nix, static
binary), `runtime` (Swarm, Kubernetes, Fly, Cloud Run, systemd), `ci`
(GitHub Actions, GitLab), `edge` (Caddy, Traefik, managed), `secrets`
(platform secrets, SOPS, Doppler), and `layout` (monorepo, polyrepo):
plugins emit units, the layout plugin decides where they live.
`http-service@1`'s `BUILD_CONTEXT`/`DOCKERFILE` move to a packaging
contract. Multiple plugins per kind are allowed where contracts permit.

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
