# Contracts

A contract is the interface between Kiln plugins. A provider fills in its
fields; a consumer is generated once per provider without knowing which
plugin that is. See [plugins/README.md](../plugins/README.md#contracts) for
how plugins provide and consume them.

| Contract | Provided by | Consumed by | What it is |
|---|---|---|---|
| [`http-service@2`](http-service@2.yaml) | backends | edge (routes), mobile (`API_*`) | A service serving HTTP on one port |
| [`oci-image@1`](oci-image@1.yaml) | backends | ci (build, push), runtime (run, verify) | A container image built from the project |
| [`migration@1`](migration@1.yaml) | backends | runtime | A one-off command run before each rollout |
| [`env@1`](env@1.yaml) | anything needing a runtime variable | ci (writes `.env`), backends (container env) | One application environment variable |
| [`config-file@1`](config-file@1.yaml) | edge | runtime (versioned configs) | A project file a service reads at runtime |
| [`image-registry@1`](image-registry@1.yaml) | registry | ci (push), runtime (pull) | Where images live, and how to log in |
| [`deploy-target@1`](deploy-target@1.yaml) | runtime | ci | The command that deploys the project |
| [`deploy-secret@1`](deploy-secret@1.yaml) | runtime | ci (exports it) | A secret the deploy command needs |
| [`go-backend@1`](go-backend@1.yaml) | Go backends | payments providers (adapter code) | A Go backend's extension slots |
| [`payment-provider@1`](payment-provider@1.yaml) | payments | backends (generate the payments package) | A payment provider adapter exists |

`http-service@1` (which also carried build fields, now `oci-image@1`) was
removed before any plugin outside this repository depended on it.

## Format

`<name>@<major>.yaml`, named after its own ID:

```yaml
name: http-service
version: 1
description: What a provider promises, and what consumers do with it.
fields:
  - name: SERVICE_NAME           # UPPER_SNAKE — used as {{SERVICE_NAME}} in binding fragments
    description: …
    required: true
  - name: PATH_PREFIX
    description: …
    default: /                   # optional fields always have a value, possibly ""
```

## Changing a contract

Contracts are the stability boundary for every plugin, in-tree or not:

- **Adding an optional field** (with a default) is backward compatible —
  existing providers keep working, existing consumers ignore it.
- **Anything else** — a new required field, a removed or renamed field, a
  change in meaning — is a new major version: add `<name>@<major+1>.yaml`
  alongside the old one and migrate plugins over. Never edit a published
  version incompatibly.

List the field that identifies a provider first (e.g. `SERVICE_NAME`,
`ENV_NAME`): Kiln uses it to label bindings in `kiln plan` output. Keep a
field's position stable across a contract version.

A field's meaning belongs in its `description`; a consumer can only rely on
what the description promises.
