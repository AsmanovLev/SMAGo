# SMAXr — architecture

> Self-modifying AI agent, channel-agnostic backend, multi-messenger UI.
> Rewrite of SMAGo (Go) in Elixir.

## Repos

```
konsolidator/      — multi-messenger adapter library (Hex package)
smaxr/             — the agent (depends on konsolidator)
```

## System overview

```
User (Telegram / VK / MAX / Discord / Web / ...)
  │
  ▼
Konsolidator.Adapters.<Name>   ← long-poll / webhook / gateway
  │  publish_incoming()
  ▼
Konsolidator.Router (Phoenix.PubSub)
  │  topic: "incoming"
  ▼
Smaxr.Router (subscribed)
  │  casts to agent
  ▼
Smaxr.Agent (one GenServer per user_id)
  │
  ├── Smaxr.LLM.OpenAI  (Req → OpenAI-compatible API)
  ├── Smaxr.Tools.*     (16+ tools, channel-agnostic)
  ├── Smaxr.DCP         (Dynamic Context Pruning)
  ├── Smaxr.Store       (:dets — sessions + dcp state)
  ├── Smaxr.Manifest    (SHA registry for eval safety)
  ├── Smaxr.Watchdog    (5s pings, panic after 3 fails)
  └── Smaxr.Recover     (Code.eval_file + restart)

Agent reply:
  Konsolidator.deliver(user_id, %Content{text, parse_mode, buttons, ...})
    │  Phoenix.PubSub broadcast on "user:USER_ID"
    ▼
  Each adapter subscribed → translates to native API
```

## Backend / Frontend split

**Backend (SMAXr):** channel-agnostic. Knows nothing about Telegram/Discord/etc.
Only emits/receives via `Konsolidator` facade.

**Frontend (Konsolidator + adapters):** channel-specific. Each adapter:

1. Implements `Konsolidator.Adapter` behaviour (8 callbacks)
2. Declares `capabilities()` (which features it supports)
3. Runs as a GenServer under `Konsolidator.Supervisor`
4. Subscribes to PubSub topics for user_ids it handles
5. Publishes incoming events via `Konsolidator.Router.publish_incoming/1`

## Adapter behaviour (8 methods)

```elixir
@callback name() :: atom()
@callback capabilities() :: [capability()]
@callback start_link(keyword()) :: GenServer.on_start()
@callback send(adapter(), user_id(), Content.t()) :: {:ok, ref()} | {:error, term()}
@callback edit(adapter(), user_id(), ref(), Content.t()) :: :ok | {:error, term()}
@callback delete(adapter(), user_id(), ref()) :: :ok | {:error, term()}
@callback typing(adapter(), user_id(), :on | :off) :: :ok
@callback answer_callback(adapter(), callback_id(), keyword()) :: :ok | {:error, term()}
```

## Common types

```elixir
%Konsolidator.Content{
  text: String.t() | nil,
  file: Path.t() | nil,
  photo: Path.t() | nil,
  video: Path.t() | nil,
  audio: Path.t() | nil,
  sticker: String.t() | nil,
  buttons: [[Button.t()]] | nil,
  parse_mode: :plain | :markdown | :html,
  reply_to: ref() | nil,
  thread: term() | nil,
  silent: boolean()
}

%Konsolidator.Button{
  label: String.t(),
  data: String.t() | nil,   # callback button
  url: String.t() | nil,     # url button (mutually exclusive with data)
  style: :default | :positive | :negative | :primary | :secondary
}
```

## User identity

An opaque `user_id` (integer or string). Same user on different channels:

- **Telegram:** user_id = Telegram chat_id (int64)
- **Web:** user_id = 1 (fixed, single-user)
- **Discord/VK/MAX:** user_id = platform-native unique id

The same `user_id` on multiple channels = same history across channels.
Konsolidator delivers to ALL adapters registered for that `user_id`.

## PubSub event flow

**Outgoing (backend → adapters):**

| Event | Topic | Description |
|---|---|---|
| `{:deliver, user_id, %Content{}}` | `"user:#{user_id}"` | New message to send |
| `{:edit, user_id, ref, %Content{}}` | `"user:#{user_id}"` | Edit existing message |
| `{:delete, user_id, ref}` | `"user:#{user_id}"` | Delete message |
| `{:typing, user_id, :on\|:off}` | `"user:#{user_id}"` | Typing indicator |

**Incoming (adapter → backend):**

| Event | Topic | Description |
|---|---|---|
| `{:incoming, %{source:, user_id:, text:, ref:, raw:}}` | `"incoming"` | User message |
| `{:incoming, %{source:, user_id:, callback_id:, button_data:, ref:}}` | `"incoming"` | Button press |

## Capabilities

Each adapter declares what it supports. Backend doesn't check capabilities
before emitting events — adapters are responsible for fallback.

Universal (all modern messengers):
`:send_text`, `:edit_text`, `:delete_message`, `:send_file`, `:send_photo`,
`:inline_buttons`, `:edit_buttons`, `:url_buttons`, `:typing_indicator`,
`:reply_to`

Optional:
`:send_video`, `:send_audio`, `:send_sticker`, `:reactions`, `:threads`,
`:forward`, `:html`, `:markdown`, `:rich_text`, `:code_blocks`,
`:bot_commands`, `:persistent_keyboard`, `:payments`, `:polls`

## Adapter priority

Based on messenger research (June 2026):

| # | Messenger | RU MAU | Global MAU | Priority | Notes |
|---|-----------|--------|------------|----------|-------|
| 1 | Telegram | ~90M | ~950M | ✅ Done | @asmanovlev_bot |
| 2 | Discord | ~4M | ~200M | High | Gateway WS, Components V2 |
| 3 | VK Messenger | ~73M | ~73M | High | Long Poll, callback buttons |
| 4 | MAX (ex-TamTam) | ~30M | ~30M | High | TG-like API, gov-backed |
| 5 | Matrix/Element | ~0.2M | ~70M | Medium | Open protocol, self-host |
| 6 | Slack | ~1M | ~80M WAU | Medium | Block Kit, enterprise |
| 7 | WhatsApp | ~97M | ~3B | Low | Billed, 24hr window |
| 8 | Viber | ~7M | ~250M reg | Low | Billed, no edit/buttons |

Skip: iMessage (deprecated ABC), Signal (no bot API), WeChat (CN entity),
Snapchat (no chat API), TikTok DMs (no API).

## Telegram adapter (reference impl)

- **Long-poll** `getUpdates` (fallback to 5s backoff on error)
- **SOCKS5 proxy** via curl subprocess (Russia: api.telegram.org blocked)
- **Markdown → TG HTML** via `Konsolidator.Adapters.Telegram.Format`
- **Inline keyboards** from `Content.buttons`
- **File upload** by path (Req detects binary paths as multipart)
- **Auto-restart** poller on crash

## LLM loop

`Smaxr.LLM.OpenAI` — OpenAI-compatible via Req (or curl for SOCKS proxy).

**Config:**
```elixir
config :smaxr, Smaxr.LLM.OpenAI,
  base_url: "https://opencode.ai/zen/go/v1",
  api_key: "sk-...",
  default_model: "kimi-k2.6",
  proxy: "socks5h://127.0.0.1:10808"
```

**Flow:**
1. Agent receives user message
2. Builds message history (SystemPrompt + session messages)
3. Calls LLM with `default_model` (or per-session override)
4. LLM returns assistant text + optional tool_calls
5. Agent executes tools → feeds results back → LLM continues
6. Loop until assistant returns plain text → deliver to user

**Tools** (channel-agnostic, from SMAGo):
`terminal`, `read_file`, `write_file`, `edit_file`, `list_dir`, `delete_file`,
`find_files`, `file_info`, `move_file`, `diff`, `grep`, `web_search`,
`vision`, `send_file`, `compress`, `eval`, `commit`

## Self-modify (eval-based)

Unlike SMAGo's binary-swap, SMAXr uses `Code.eval_string/3` in the running
BEAM. Protection layers:

1. **SystemManifest** — SHA registry of all `lib/smaxr/*.ex` files
2. **Watchdog** — pings critical GenServers every 5s, panics after 3 failures
3. **Recover** — `Code.purge` + `Code.eval_file(.ex)` + Application.stop/start
4. **Sandbox** — AST inspection for forbidden calls (halt, rm_rf, os.cmd, etc.)

## Storage

`Smaxr.Store` wraps `:dets` (stdlib, no NIF). Tables:
- `sessions.dets` — conversation history per user/session
- `dcp.dets` — DCP state per user
- Audit logs to `priv/smaxr/audit/eval.jsonl` and `commits.jsonl`

In-memory (ETS): step counts, trace buffers, file hashes.

## Config

```json
{
  "telegramToken": "BotFather token",
  "telegramChatID": 123456789,
  "provider": "opencode-go",
  "defaultModel": "kimi-k2.6",
  "dataDir": "./priv/smaxr",
  "trustedChatIDs": [123456789],
  "systemPrompt": "...",
  "providers": { ... },
  "mcp": { ... },
  "deltachat": { "enabled": true, "email": "...", "password": "..." }
}
```

## Tools (16+ ported)

| Tool | Description | Test |
|------|-------------|------|
| `terminal` | Execute shell command | ✅ |
| `read_file` | Read file content | ✅ |
| `write_file` | Write content to file | ✅ |
| `edit_file` | Replace string in file | untested |
| `list_dir` | List directory entries | untested |
| `delete_file` | Delete file/dir | untested |
| `file_info` | File metadata | ✅ |
| `move_file` | Rename/move file | untested |
| `diff` | Show file differences | untested |
| `grep` | Search pattern in files | untested |
| `find_files` | Find by glob | untested |
| `web_search` | DuckDuckGo HTML parse | untested |
| `vision` | Analyze image via LLM | untested |
| `send_file` | Stage file for delivery | untested |
| `eval` | Run Elixir code (sandboxed, 15s timeout) | untested |
| `commit` | Git commit all changes | untested |
| `compress` | Receives text | untested |

## Test status

```
konsolidator: 69 tests, all green
smaxr:        28 tests, all green
```

## Git history

```
konsolidator:
  1bc6698 Initial mix new konsolidator
  1c53136 Add core data types, behaviour, registry, router
  162a993 Add Telegram adapter, format, api, poller with tests
  0b27d06 Fix clause grouping warnings in Telegram adapter
  1b384c4 Add SOCKS proxy via :httpc, live smoke test passes
  e956b8b Add SOCKS proxy (curl fallback), fix method camelCase, real bot integration passes

smaxr:
  dc0c8e7 Initial SMAXr skeleton: agent, router, application, tests
  06eafb5 Add STATUS.md
  1b70e1f Fix clause grouping, add smoke integration script
  2ecaedf Real Telegram integration test passes, dev config with SOCKS proxy
  d99a637 Wire LLM into Agent, all 19 tests green
  <commit> Port 16+ tools from SMAGo, add tool tests (9), all 28 green
```

## File map

```
konsolidator/
├── mix.exs           — 2 deps: req, phoenix_pubsub
├── lib/konsolidator/
│   ├── adapter.ex        — @behaviour (8 callbacks)
│   ├── content.ex        — Content struct
│   ├── button.ex         — Button struct
│   ├── capabilities.ex   — capability types
│   ├── registry.ex       — GenServer registry
│   ├── router.ex         — PubSub routing
│   ├── supervisor.ex     — DynamicSupervisor for adapters
│   ├── contract.ex       — test macros
│   └── adapters/telegram/
│       ├── telegram.ex   — main GenServer
│       ├── format.ex     — MD → TG HTML
│       ├── api.ex        — HTTP calls (Req + curl-SOCKS)
│       └── poller.ex     — long-poll getUpdates

smaxr/
├── mix.exs           — 3 deps: konsolidator, req, phoenix_pubsub
├── config/
│   ├── config.exs     — Telegram + proxy defaults
│   ├── dev.exs        — real tokens, LLM config
│   └── test.exs       — no adapters, fake LLM URL
├── lib/smaxr/
│   ├── application.ex — supervisor tree
│   ├── router.ex      — incoming PubSub subscriber
│   ├── agent.ex       — per-user GenServer
│   ├── agent/
│   │   └── supervisor.ex  — DynamicSupervisor
│   ├── llm.ex         — @behaviour
│   └── llm/
│       ├── message.ex — Chat message struct
│       └── openai.ex  — OpenAI-compatible impl
├── scripts/
│   ├── smoke_integration.exs       — SMAXr + FakeTelegram integration
│   └── smoke_real_integration.exs  — SMAXr + real Telegram bot
└── test/
    ├── smaxr/agent_test.exs
    ├── smaxr/llm/*_test.exs
    └── smaxr_test.exs

messenger_research.md  — full capability matrix, 603 lines
```

## Dependencies (5 total)

| Package | Used for | In |
|---------|----------|----|
| `req` | HTTP client (LLM, Telegram API) | konsolidator + smaxr |
| `phoenix_pubsub` | PubSub between backend and adapters | konsolidator + smaxr |
| `jason` | JSON encode/decode (transitive via req) | konsolidator + smaxr |
| `finch` | HTTP/2 pool (transitive via req) | konsolidator + smaxr |
| `mint` | HTTP/1.1 (transitive via req) | konsolidator + smaxr |

## Symlink from SMAGo

```
SMAGo (Go)           →  main branch
smago_elixir (this)  →  branch SMAXr
konsolidator         →  separate repo (eventually hex.pm)
```
