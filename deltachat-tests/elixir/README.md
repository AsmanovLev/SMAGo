# DeltaChat standalone smoke-test (Elixir).

Stdlib + Jason. Spawns `deltachat-rpc-server.exe` via Port, speaks
newline-delimited JSON-RPC, polls events via `get_next_event`.

## Run

```powershell
mix deps.get
mix run --no-halt
```

Expect one line: `[ex] OK`.

## Requires

- Elixir 1.20+ with Erlang/OTP 27/28/29
- `deltachat-rpc-server.exe` reachable as `deltachat-rpc-server` in PATH

## Source

- `lib/dc_test.ex` — test orchestrator (read common.json, invite.txt, run sequence)
- `lib/rpc.ex` — JSON-RPC client GenServer (stdin framing + stdout demultiplexer)

See `../PROTOCOL.md` for the wire format this code implements.
