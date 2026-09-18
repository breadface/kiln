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

See [plugins/README.md](plugins/README.md) for the manifest format, the
`files/`/`fragments/` split, and the filename escapes required because
plugin content is embedded via `//go:embed` (`.tmpl` suffix, `dot__` prefix
— read that section before adding a `.go` file or a dotfile to a plugin, or
it will silently fail to embed).

After adding or changing plugin content, regenerate a project and check it
by hand — `docker compose config` (and, for the hosting plugin,
`docker compose -f docker-compose.yml -f docker-compose.swarm.yml config`)
catches Compose schema and merge mistakes that a Go test won't.

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
