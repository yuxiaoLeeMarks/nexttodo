# NextTodo

A Mac planner for researchers, in Go with MyGo's native UI (no webview), in English and 简体中文. Its assistant and translator are the user's **pi** agent, run in RPC mode. How the parts fit, threads and pi's protocol included: [docs/architecture.md](docs/architecture.md). Read it before touching `agent*.go`, `assistant.go`, `translate.go` or `internal/`.

## Working on it

- **Strings**: every string the user sees lives in `locales/en.json` and `locales/zh-Hans.json`, both, then `go tool mygo generate` rewrites `locales/catalog_gen.go`. The app's Chinese reads as written Chinese, not a translation.
- **Look**: magpie's (soft gray window, white cards with hairline edges, one indigo accent), with the palette and components in `style.go`. Reuse them. The window's smallest size is 940×600.
- **Feel first**: smoothness and an answer to every click come before CPU or memory. Nothing slow on the main thread: work on a goroutine, hand back with `mygo.RunOnMain` (it blocks until run), and a stream of events through `app.inbox`.
- **Data**: changes run on the main thread and end with `a.changed()`. A change the user may regret is an undo step (`remember`/`settle`). A tool of the assistant's that writes goes through `agentChange`, so its run can undo it. The assistant has no tool that deletes, by design.

## Checking it

- `go vet ./... && go test -race ./...`, and the same under `TZ=America/New_York` (clock changes).
- See a page: `NT_SNAPSHOTS=/tmp/s NT_PAGE=<shot names> go test -run TestSnapshots .`, with `NT_SIZE=940x600` for the smallest window. These render headless. Use them for layout, not the running app's capture, which shows a stale frame when the window is covered.
- The real pi: `NT_REAL_PI=1 go test -run TestRealPi .` (what tools it reaches), or the built app with `NEXTTODO_DATA=/tmp/<dir>/data.json NEXTTODO_CAPTURE=<dir> NEXTTODO_ASK="…"` (see `checkAgent` in `main.go`). Always point `NEXTTODO_DATA` at a scratch file; the user's own data is in `~/Library/Application Support/NextTodo`.
- pi's credentials (`~/.pi/agent/auth.json`, `mcp-auth.json`, keys in `models.json`) stay unread and unprinted.

## Releasing

Bump `version` in `mygo.json` and the fallback in `appVersion()` (`settings.go`) together. Add a `## <version>` section at the top of `CHANGELOG.md`, in English then Chinese, as the earlier ones. The release workflow takes its notes from that section. Push a `v<version>` tag: `.github/workflows/release.yml` builds, signs the update with the repo secret, and publishes.
