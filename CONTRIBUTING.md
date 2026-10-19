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
  --backend go-api \
  --module github.com/you/demoapp-api \
  --domain demo.example.com \
  --admin-email you@example.com \
  --dir /tmp/kiln-try --yes
```

## Adding or changing a plugin

Every supported plugin combination is listed in `scenarios` in
`internal/cli/apply_test.go`, and snapshotted in
`internal/cli/testdata/plans/<scenario>.golden`: the plan summary, a hash
per generated file, and the full content of every file plugins merge
fragments into. After an intended plugin change, regenerate the snapshots
and commit them with the change, so reviewers see exactly what it does to
each combination:

```sh
go test ./internal/cli -run TestGoldenPlans -update
git diff internal/cli/testdata/plans
```

A new plugin adds its combinations to `scenarios`. Bump a plugin's
`version` whenever its content changes — `kiln apply` treats a changed
plugin as a different plan.

See [plugins/README.md](plugins/README.md) for the manifest format,
contracts, the `files/`/`fragments/`/`bindings/` split, and the filename escapes required because
plugin content is embedded via `//go:embed` (`.tmpl` suffix, `dot__` prefix
— read that section before adding a `.go` file or a dotfile to a plugin, or
it will silently fail to embed).

After adding or changing plugin content, regenerate a project and check it
by hand — `docker compose config` (and, for the hosting plugin,
`docker compose -f docker-compose.yml -f docker-compose.swarm.yml config`)
catches Compose schema and merge mistakes that a Go test won't. For the
hosting plugin's other generated files:

```sh
docker run --rm -v "$PWD/Caddyfile:/etc/caddy/Caddyfile:ro" caddy:2-alpine \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker run --rm -v "$PWD:/repo" -w /repo rhysd/actionlint:1.7.7 .github/workflows/deploy.yml
```

To change a contract, read [contracts/README.md](contracts/README.md) first
— contracts are versioned, and an incompatible change is a new version.

For the mobile plugin specifically, the Go test suite only checks structural
things (no leftover placeholders, the right directory layout, the
`gradle-wrapper.jar` round trip) — it doesn't invoke Gradle, Xcode, or an
Android SDK. After changing `plugins/mobile/kmp-compose/`, regenerate a
project with `--backend go-api --mobile kmp-compose --mobile-package
com.example.demoapp` and, if you have the toolchain installed, verify by
hand:

```sh
cd <generated>/demoapp-mobile
./gradlew :composeApp:assembleDebug          # real Android debug build
cd iosApp && xcodebuild -project iosApp.xcodeproj -scheme iosApp \
  -sdk iphonesimulator -destination 'id=<a-simulator-udid>' build
```

`xcrun simctl list devices` prints simulator UDIDs. Both are real, working
build invocations — this is how the plugin's initial content was verified,
including catching a missing Xcode Run Script build phase that only a real
`xcodebuild` run surfaces.

## Commit conventions

Commits are atomic and scoped to one logical change — prefer several small,
buildable commits over one large one. Messages follow a rough
`type(scope): summary` shape (`feat`, `fix`, `docs`, `test`, `chore`, `ci`),
matching the existing history (`git log --oneline`). Explain *why* in the
body when the commit fixes something non-obvious, not just what changed.

## Documentation

Update the relevant `README.md` (root or `plugins/`) in the same change as
the code it describes, not as a follow-up.
