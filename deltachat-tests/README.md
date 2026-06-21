# deltachat-tests

Standalone smoke-test for DeltaChat integration. Three identical test clients in three languages, all talking to the same `deltachat-rpc-server.exe` (v2.53.0+).

The point: **verify that DeltaChat works through each binding** (Go official, Python stdlib JSON-RPC, Elixir stdlib JSON-RPC) using the same login (`sma.go@ro.ru` on rambler.ru) and the same target (secure-join to `delt.er@bk.ru` / "Delter").

## Layout

```
deltachat-tests/
├── common.json     login: email, password, display name
├── invite.txt      secure-join URL for Delter
├── PROTOCOL.md     JSON-RPC protocol reference (frame format, methods, events)
├── README.md       this file
├── go/             Go client (uses deltachat-rpc-client-go v1.134.0)
├── python/         Python client (stdlib only, raw JSON-RPC frames)
├── elixir/         Elixir client (Jason + Port, raw JSON-RPC frames)
└── _runtime/       isolated per-run sqlite db (gitignored)
```

## Run all three

From `D:\Users\User\Desktop\SMAGo\` (any cwd, scripts use absolute paths internally):

```powershell
cd deltachat-tests\go     && go run .
cd deltachat-tests\python && python test.py
cd deltachat-tests\elixir && mix deps.get && mix run --no-halt
```

Each prints a single line `OK` and exits 0 on success, or `FAIL: <reason>` and exits 1.

## Message sent

```
Hello from {go|py|ex} test, SMAGo deltachat {go|py|ex} smoke test, {ISO8601 timestamp}
```

Three messages will land in the chat with "Delter" / `delt.er@bk.ru`.

## Requirements

- Go 1.26+ (already installed)
- Python 3.13+ with `deltachat-rpc-server` in PATH (already installed via pip)
- Elixir 1.20+ with OTP 27/28/29 (install via `winget install Erlang.ErlangOTP` + download `elixir-otp-29.zip` from GitHub releases)
- All three need `deltachat-rpc-server.exe` reachable as `deltachat-rpc-server` in PATH

## Runtime isolation

Each test creates `_runtime/deltachat-db/` in this directory. It does **not** touch `data/deltachat-db/` (the production SMAGo account). Safe to delete `_runtime/` between runs.

## Security note

`common.json` contains real credentials. `.gitignore` does **not** exclude it on purpose — if you copy this directory elsewhere, scrub the password first.
