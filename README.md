# Kiln

Kiln is a CLI that scaffolds new projects by composing independent, reusable
plugins — a backend, a hosting setup, a payment provider, and so on — instead
of copy-pasting boilerplate between projects.

Status: early development. `kiln new` currently composes a minimal Go API
backend with a self-hosted Docker Swarm + Caddy hosting setup.

## Install

```sh
go install github.com/surebits/kiln/cmd/kiln@latest
```

## Usage

```sh
kiln new myapp
```

More documentation is on the way as the tool takes shape.
