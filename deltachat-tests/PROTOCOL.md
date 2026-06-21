# DeltaChat JSON-RPC protocol reference

This is the wire protocol used by `deltachat-rpc-server.exe`. All three test clients in this directory (Go, Python, Elixir) speak this protocol directly or via a binding.

## Framing (newline-delimited JSON)

Server speaks JSON-RPC 2.0 over **stdio**. Each frame is a single line:

```
{"jsonrpc":"2.0","id":1,"method":"get_all_account_ids","params":[]}\n
```

- No LSP-style `Content-Length` headers
- No batching, one request per line
- Server flushes after each response
- Both directions use the same framing

## Spawn

```python
subprocess.Popen(["deltachat-rpc-server.exe"], stdin=PIPE, stdout=PIPE, env={"DC_ACCOUNTS_PATH": "..."})
```

The `DC_ACCOUNTS_PATH` env var sets the sqlite location. Without it, server uses CWD. **No CLI args** — passing `--account-dir` or similar will be rejected.

## Methods (subset we use)

All calls return `{"jsonrpc":"2.0","id":N,"result":...}` or `{"error":{...}}`.

| Method | Params | Result |
|---|---|---|
| `add_account` | `[]` | `account_id` (u32) |
| `get_all_account_ids` | `[]` | `[account_id, ...]` |
| `set_config` | `[account_id, "key", "value"]` | `null` |
| `get_config` | `[account_id, "key"]` | `"value"` |
| `configure` | `[account_id]` | `null` (or throws) |
| `start_io` | `[account_id]` | `null` |
| `stop_io` | `[account_id]` | `null` |
| `get_next_event` | `[]` | event payload (blocks until event ready, or returns `null`) |
| `secure_join` | `[account_id, "<qr-data>"]` | `chat_id` (u32) |
| `send_msg` | `[account_id, chat_id, {text: "..."}]` | `msg_id` |
| `get_chat_msgs` | `[account_id, chat_id]` | `[msg_id, ...]` |

## Config keys (subset)

| Key | Meaning |
|---|---|
| `addr` | email address |
| `mail_pw` | password / app token |
| `displayname` | account display name (**no underscore**) |
| `configured_mail_server` | override IMAP host |
| `configured_mail_port` | override IMAP port |
| `configured_mail_user` | override IMAP user |
| `configured_mail_pw` | override IMAP password |
| `configured_send_server` | override SMTP host |
| `configured_send_port` | override SMTP port |
| `configured_send_user` | override SMTP user |
| `configured_send_pw` | override SMTP password |

For non-chatmail providers (rambler.ru, gmail, etc.) you must set the `configured_*` overrides. Chatmail relays auto-configure.

## Events

**Events are NOT pushed** by the server. You must poll `get_next_event` in a long-running loop. The call **blocks** on the server side until an event is ready, then returns the event payload directly as the result:

```json
// You send:
{"jsonrpc":"2.0","id":42,"method":"get_next_event","params":[]}

// Server returns (no event yet, immediate poll):
{"jsonrpc":"2.0","id":42,"result":null}

// Server returns (event arrived):
{"jsonrpc":"2.0","id":43,"result":{"contextId":1,"event":{"kind":"ImapConnected","msg":"IMAP-LOGIN as sma.go@ro.ru"}}}
```

The result IS the event payload: `params.contextId` is the account, `params.event.kind` is the event type, and `params.event` carries event-specific fields (`msg`, `progress`, `contactId`, etc.).

Relevant event kinds:
- `Info` / `Warning` / `Error` — log messages (`event.msg`)
- `ImapConnected` / `SmtpConnected` — socket up, `event.msg` is the login status
- `ConfigureProgress` — `event.progress` (0..1000), `event.comment`
- `SecurejoinJoinerProgress` — `event.progress` (400 = verified)
- `MsgDelivered` — `event.msg_id`
- `MsgFailed` — `event.msg_id`, `event.error`

## Secure join (invite URL)

The URL we have:

```
https://i.delta.chat/#B3FF8E79CB1A4FDE37A9904CB8DE491B5799D33D&v=3&i=E0Mf3TYcnuhVaq7q9-BieUnM&s=QP-0fgLlSnE8WEBHyejlDyHO&a=delt.er%40bk.ru&n=Delter
```

The `securejoin` method takes the **raw** URL (the server parses out the fingerprint and contact address itself). The contact is `delt.er@bk.ru` (display name "Delter"). After successful secure-join the chat appears as a 1:1 chat.

## Rambler.ru specifics

```python
"configured_mail_server": "imap.rambler.ru"
"configured_mail_port": "993"
"configured_send_server": "smtp.rambler.ru"
"configured_send_port": "465"
```

Rambler is touchy about "non-standard" clients. If it refuses, try via web first to confirm the account works, then re-run.

## Sequence

1. Spawn server with `DC_ACCOUNTS_PATH` env var
2. **Spawn a long-lived polling thread that loops `get_next_event`**. This is the only way to receive events.
3. Reader goroutine/thread consumes stdout line-by-line and demultiplexes responses (by `id`) and events (from poll results)
4. `add_account` → `account_id`
5. `set_config` × N (addr, mail_pw, displayname, configured_*)
6. `configure` (this blocks until autoconfig done; can take 30s, returns events during)
7. `start_io` (returns immediately; events follow)
8. `secure_join` with QR URL
9. Wait for `SecurejoinJoinerProgress` with `progress >= 400` ("alice verified, introducing myself")
10. `send_msg` to returned chat_id
11. Wait for `MsgDelivered` event (optional)
12. `stop_io`, close pipes
