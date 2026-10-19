# Contracts

A contract is the interface between Kiln plugins. A provider fills in its
fields; a consumer is generated once per provider without knowing which
plugin that is. See [plugins/README.md](../plugins/README.md#contracts) for
how plugins provide and consume them.

| Contract | Provided by | Consumed by | What it is |
|---|---|---|---|
| [`http-service@1`](http-service@1.yaml) | backends | hosting (build, deploy, route), mobile (`API_*`) | A container-built service serving HTTP on one port |
| [`migration@1`](migration@1.yaml) | backends | hosting | A one-off command run before each rollout |
| [`env@1`](env@1.yaml) | anything needing a runtime variable | hosting (secrets) | One environment variable |

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
