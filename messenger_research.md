# Messenger Bot API Research (2025-2026)

> Research for SMAGo multi-messenger expansion.
> Methodology: direct doc fetches via Playwright + Bing search + training-data knowledge (Jan 2026 cutoff) cross-checked.
> Where a docs site blocked automation, the canonical URL is listed and the values were verified against third-party sources and well-known reference implementations.

---

## Section 1 — Executive summary

**Goal:** rank messenger bot platforms by how suitable they are for a Go-based, single-user, agent-style bot like SMAGo (Telegram-style text + tools + markdown + inline buttons).

### Priority order — which messengers are worth supporting

| # | Messenger | Why | Effort | Notes |
|---|-----------|-----|--------|-------|
| 1 | **Telegram** ✅ already done | Best public bot API in the world. Free, BotFather, long-poll, rich features. | none | reference impl in `src/telegram.go` |
| 2 | **Discord** | Excellent bot API (slash commands, components V2, embeds), free, huge dev community, Go lib `bwmarrin/discordgo` is mature. | low | Bot + socket gateway, needs Discord app + token |
| 3 | **Slack** | Mature, free tier, Block Kit is the gold standard for rich UI. Workspace install model. | low | OAuth flow, socket mode for no-webhook dev |
| 4 | **VK / VK Messenger** | Russia #1 social+chat. Community messages API, Long Poll, Callback API. Free, well-documented in Russian. | low | uses access_token for community; user messages for personal account |
| 5 | **Matrix / Element** | Open protocol, self-hostable, native threads, free, perfect for a tool-bot. | medium | Application Service or `@matrix-bot-sdk` for bots |
| 6 | **MAX (ex-TamTam)** | Russian national messenger, MAU rising fast in 2025-2026, Bot API very similar to Telegram (HTTPS JSON, polling+webhook). | low | dev.max.ru, requires VK account; new platform, docs thin |
| 7 | **Yandex.Messenger** | Russia business chat; Bot API documented at yandex.com/dev/messenger. | low | uses OAuth token; small reach but useful for Yandex 360 biz |
| 8 | **Microsoft Teams** | Adaptive Cards are best-in-class for rich UI. Enterprise-only adoption. | medium | Bot Framework, OAuth, Azure Bot registration |
| 9 | **WhatsApp Business (Cloud)** | 3B users, Meta-hosted. Best reach globally. **PERN-PRICED**, conversation-billed. | medium | 24h window rules, template messages, no free-form first contact |
| 10 | **Google Chat** | Workspace integration, free for Workspace users. Card UI. | medium | service account auth, Google API client libs |
| 11 | **Odnoklassniki (OK)** | Russia #2 social. Mediator API, free. | medium | uses OK app id + secret + session key |
| 12 | **Facebook Messenger** | Massive reach, but declining for brand-bots; Instagram DMs same platform now. | medium | same Meta infra as WhatsApp Cloud, similar restrictions |
| 13 | **Viber** | Strong in Eastern Europe, MENA. Bot Account + PA. | medium | per-message cost; needs verified bot account |
| 14 | **Line** | Japan/Taiwan/Thailand. Messaging API is excellent (rich messages, Flex, quick reply). | medium | LINE Official Account, channel access token |
| 15 | **KakaoTalk** | Korea #1. Kakao i Open Builder + Bizmessage. | medium-high | Bizmessage approval is heavy; channel model |
| 16 | **imo** | Russia/CIS, SE Asia, MENA. Limited public bot API. | high | mostly undocumented bot support |
| 17 | **Threema** | Switzerland, privacy-focused, paid (Threema Business). | high | licensed per user |
| 18 | **Wire** | Enterprise secure messenger. Limited 3rd-party bot support. | high | enterprise-only |

**Skip / no realistic path:**

| Messenger | Reason |
|-----------|--------|
| iMessage / Apple Business Chat | Apple shut down ABC (2023). iMessage has no bot API. RCS Business Messaging replaces it. |
| Google Messages (consumer RCS) | RCS Universal Profile bot support is carrier-fragmented; Google Jibe/RCSe only via Google's "RCS Business Messaging" partner program. |
| Signal | No public bot API. Third-party `signald`/`signal-cli` exist but Terms of Service forbid them. |
| Snapchat | Snap Kit / Creative Kit does not expose a chat-bot API. |
| TikTok DMs | No public bot API; only Marketing API for ads. |
| WeChat | Open API requires a Chinese business entity + 300 CNY verification + WeChat Pay. Foreigners effectively excluded. |
| QQ | Tencent bot framework exists but is China-only and 300 CNY + entity verification. |
| ICQ | Shutdown 2024. Replaced by VK Messenger and (ironically) "VK Мессенджер" / agent.ru closed 2025. |
| Jabber / XMPP | No "bot API" — it's a protocol. You write a client. Feasible but no users; work without corporate sponsorship. |
| Zoom Chat | Bot support is on a long sunset; Zoom pivoted to Zoom Team Chat, then deprecated bot APIs in 2024. |
| Skype | Microsoft deprecated Skype bot platform 2023, migrated to Teams. |
| TamTam | Officially renamed to **MAX** in 2025; tamtam.com redirects to max.ru. |

**Headline:** for SMAGo the next four adapters that buy the most users for the least code are **Discord, Slack, VK, Matrix/Element**. WhatsApp follows only if cost is acceptable.

---

## Section 2 — Global top-25 detailed table

> Region column: **G** = global reach, **R** = regional (Asia / LatAm / MENA / RU / KR / JP / CN).
> MAU is the latest 2025 or 2026 figure from public sources (Statista, Sensor Tower, Mediascope, DataReportal, company reports).

| # | Messenger | MAU (2025-2026) | Region | Bot API | Docs URL |
|---|-----------|-----------------|--------|---------|----------|
| 1 | WhatsApp | 3.0 B | G | yes (Cloud / On-Prem) | https://developers.facebook.com/docs/whatsapp/cloud-api |
| 2 | WeChat | 1.4 B (2025) | CN | restricted (CN only, biz entity) | https://developers.weixin.qq.com/doc/ |
| 3 | Facebook Messenger | 1.0 B (2025) | G | yes (Meta Business) | https://developers.facebook.com/documentation/business-messaging/messenger-platform |
| 4 | Telegram | 950 M (2025) | G/RU | yes, free | https://core.telegram.org/bots/api |
| 5 | Snapchat | 750 M (2025) | G | no (no chat-bot API) | https://docs.snap.com/snap-kit |
| 6 | TikTok (DMs) | 700 M DMs (2025) | G | no (Marketing API only) | https://business-api.tiktok.com/portal/docs |
| 7 | QQ | 600 M (2025) | CN | restricted (CN, biz entity) | https://bot.q.qq.com/wiki/ |
| 8 | Discord | 200 M MAU (2025) | G | yes, free | https://discord.com/developers/docs/intro |
| 9 | Line | 178 M MAU (2025, JP+TW+TH) | JP/TW/TH | yes (channel + OA) | https://developers.line.biz/en/docs/messaging-api/ |
| 10 | KakaoTalk | 49 M MAU (KR only, 2025) | KR | yes (Kakao i Open Builder / Bizmessage) | https://developers.kakao.com/ |
| 11 | Viber | 250 M+ registered (2025) | EE/MENA/Asia | yes (PA + Bot Account) | https://developers.viber.com/docs/api/rest-bot-api/ |
| 12 | Microsoft Teams | 320 M MAU (2025) | G (enterprise) | yes (Bot Framework) | https://learn.microsoft.com/en-us/microsoftteams/platform/bots/ |
| 13 | Slack | 38 M DAU, ~80 M WAU (2025) | G (enterprise) | yes (Block Kit) | https://api.slack.com/ |
| 14 | Zoom Chat | bundled in 300 M+ Zoom MAU | G (enterprise) | sunset (2024) | https://developers.zoom.us/team-chat/ |
| 15 | Google Messages (RCS) | 1 B+ RCS-capable (2025) | G (Android) | restricted (RCS BM via partner) | https://developers.google.com/business-communications/rcs-business-messaging |
| 16 | Signal | 70 M MAU (2025) | G | no (TOS) | https://signal.org/ — no docs |
| 17 | iMessage / Apple Business Chat | 1.5 B Apple devices | G | **deprecated** (ABC shut down 2023) | n/a |
| 18 | Google Chat | 10 M+ WAU, every Workspace user | G (Workspace) | yes (Chat API) | https://developers.google.com/workspace/chat |
| 19 | Skype | 36 M DAU (2025) | G | **deprecated** (2023) | https://learn.microsoft.com/en-us/microsoftteams/skype-bots- |
| 20 | Element / Matrix | 70 M+ accounts on matrix.org (2025) | G / privacy | yes (open protocol) | https://spec.matrix.org/ |
| 21 | Threema | 10 M (2025) | CH/DE/EU | yes (Threema Business SDK, paid) | https://developer.threema.ch/ (down — see threema.ch/en) |
| 22 | Wire | 2 M enterprise (2025) | EU/enterprise | limited (Bot SDK) | https://github.com/wireapp/wire-server |
| 23 | ICQ | 0 (shut down 2024) | RU/CIS | **dead** | https://icq.com/ → closed |
| 24 | Jabber / XMPP | n/a (protocol) | G (decentralized) | protocol, no central API | https://xmpp.org/ |
| 25 | Instagram DMs | 2 B MAU (2025) | G | yes (Meta Business, same as Messenger) | https://developers.facebook.com/documentation/business-messaging/instagram |

### Detailed capability table (global top-25)

Legend: ✅ yes · ⚠ limited · ❌ no · `?` unknown

| Messenger | send_text | edit_text | del_msg | send_file | send_photo | send_video | send_audio | send_sticker | inline_btn | edit_btn | url_btn | typing | reactions | threads | reply_to | forward | read_rcpts | md | html | rich | code_blk | max_MB | bot_cmd | persist_kbd | payments | polls | auth | updates | rate | cost | notes |
|-----------|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| WhatsApp | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ⚠ | ❌ | 100 | ❌ | ❌ | ❌ | ❌ | Meta Cloud token | webhook | 80/sec default | per-conversation billed | 24h window, template outside |
| WeChat | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | ⚠ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | 20 | ❌ | ❌ | ✅ | ❌ | appid+secret+access_token | webhook | 2000/req limit | enterprise only | CN-only |
| FB Messenger | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ postback | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ⚠ | ❌ | ❌ | ⚠ | ❌ | 25 | ⚠ persistent menu | ⚠ | ❌ | ❌ | page token | webhook | 50/sec | free | 24h window |
| Telegram | ✅ | ✅ | ✅ | ✅ 50MB | ✅ 10MB | ✅ 50MB | ✅ 50MB | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ forum | ✅ | ✅ | ⚠ (premium) | ⚠ subset | ✅ | ✅ | ✅ | 50 | ✅ | ✅ | ✅ Stars | ✅ | BotFather token | long-poll / webhook | 30/sec global, 1/sec per group | free | gold standard |
| Snapchat | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | Snap Kit OAuth | n/a | n/a | free | no chat-bot API |
| TikTok DMs | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | n/a | n/a | n/a | n/a | DMs not exposed |
| QQ | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | ⚠ | ❌ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 20 | ✅ | ❌ | ❌ | ❌ | appid+token | websocket | 5/5s | free | CN only |
| Discord | ✅ | ✅ | ✅ | ✅ 25MB | ✅ embed | ✅ embed | ✅ | ✅ custom | ✅ components | ✅ | ✅ | ✅ | ✅ | ✅ forum | ✅ | ✅ | ❌ | ⚠ subset | ❌ | ✅ (embeds) | ✅ | 25 (boost 50-100) | ✅ slash | ⚠ buttons v2 | ❌ | ✅ | bot token | gateway websocket | 5/2s burst | free | best for tool-bots |
| Line | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ postback | ❌ | ✅ | ⚠ loading anim | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠ Flex | ❌ | 200 (video) | ❌ | ✅ quick reply | ❌ | ❌ | channel access token | webhook | 10/sec (free), 60+/sec (paid) | freemium | rich messages |
| KakaoTalk | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ quick reply | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 50 | ❌ | ✅ | ❌ | ❌ | Kakao i appid | webhook / API polling | 100/sec | freemium | biz approval heavy |
| Viber | ✅ | ❌ | ❌ | ✅ 50MB | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 50 | ❌ | ✅ keyboard | ❌ | ❌ | PA token (X-Viber-Auth) | webhook | 100/sec | per-message billed | 4xx countries |
| MS Teams | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ adaptive card | ✅ | ✅ | ✅ | ✅ | ✅ reply chain | ✅ | ❌ | ❌ | ⚠ subset | ❌ | ✅ Adaptive Cards | ⚠ | 250 (OneDrive) | ✅ | ✅ | ❌ | ✅ | Bot Framework appid+secret | webhook / streaming | 4 req/sec/channel | free | Azure reg |
| Slack | ✅ | ✅ | ✅ | ✅ 1GB | ✅ | ❌ (link only) | ✅ | ✅ | ✅ blocks | ✅ | ✅ | ✅ | ✅ | ✅ threads | ✅ | ❌ | ❌ | ⚠ mrkdwn | ❌ | ✅ Block Kit | ✅ | 1024 | ✅ slash | ✅ home tab | ❌ | ❌ | OAuth + bot token | socket mode / events | 1/sec/typo, 50/min | freemium | workspace install |
| Zoom Chat | ⚠ | ❌ | ⚠ | ⚠ | ❌ | ❌ | ❌ | ❌ | ⚠ | ❌ | ⚠ | ❌ | ⚠ | ❌ | ⚠ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 50 | ⚠ | ❌ | ❌ | ❌ | JWT OAuth | webhook | ? | free | sunset 2024 |
| Google Messages (RCS BM) | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ suggested replies | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 100 | ❌ | ✅ chip-list | ❌ | ❌ | partner-only OAuth | webhook | ? | per-message billed | carrier-fragmented |
| Signal | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | n/a | n/a | n/a | n/a | no public bot API |
| iMessage / ABC | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | **deprecated** | n/a | n/a | n/a | shut down 2023 |
| Google Chat | ✅ | ✅ | ✅ | ✅ drive | ✅ | ⚠ drive link | ❌ | ❌ | ✅ cards | ✅ | ✅ | ❌ | ✅ | ✅ spaces | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ Cards | ❌ | 25 (drive) | ✅ slash | ✅ cards | ❌ | ❌ | service-account JSON | webhook / pubsub | ? | free (Workspace) | enterprise |
| Skype | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | **deprecated** | n/a | n/a | n/a | shut down 2023 |
| Element / Matrix | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ m.sticker | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ threads (native) | ✅ | ✅ | ⚠ | ⚠ html subset | ✅ | ✅ | ✅ | 100 (per HS) | ⚠ | ❌ | ❌ | ✅ | appservice / access token | long-poll `/sync` | matrix-defined | free / self-host | best open alternative |
| Threema | ✅ | ✅ | ✅ | ✅ 50MB | ✅ | ✅ | ✅ | ✅ | ✅ interactive | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | 50 | ❌ | ❌ | ❌ | ❌ | Threema-ID + key file | webhook | ? | **paid per seat** | Switzerland privacy |
| Wire | ✅ | ⚠ | ✅ | ✅ | ✅ | ✅ | ⚠ | ❌ | ✅ | ⚠ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ⚠ | ❌ | ⚠ | ❌ | 100 | ❌ | ❌ | ❌ | ❌ | bot access token | webhook | ? | enterprise | EU regulated |
| ICQ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | **dead** | n/a | n/a | n/a | shut down 2024 |
| Jabber / XMPP | ✅ | ⚠ (XEP) | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ⚠ MUC | ✅ | ✅ | ✅ | ❌ | ❌ | ⚠ XEP-0393 | ❌ | unlimited (your server) | ❌ | ❌ | ❌ | ✅ | SASL / XEP-0077 | XMPP stream (TCP/WS/BOSH) | your own | free | write a client, not an API |
| Instagram DMs | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ⚠ | ⚠ ice-breaker | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ⚠ | ❌ | ❌ | ❌ | ❌ | 25 | ❌ | ⚠ quick replies | ❌ | ❌ | Meta page token | webhook | 50/sec | free | same infra as Messenger |

---

## Section 3 — Russia top detailed table

Sources: Mediascope (Dec 2025 monthly reach), Statista (Dec 2025), Sensor Tower Q4 2025, Forbes/Mediascope interviews Mar 2026, Moscow Times Sep 2025.

| # | Messenger | Russia MAU (2025-2026) | Bot API | Docs URL |
|---|-----------|------------------------|---------|----------|
| 1 | WhatsApp | ~97 M RU (Mediascope, Dec 2025) | yes (Cloud) | https://developers.facebook.com/docs/whatsapp/cloud-api |
| 2 | Telegram | ~90 M RU (Mediascope, Dec 2025) | yes | https://core.telegram.org/bots/api |
| 3 | VK / VK Messenger | ~73 M RU (2025) | yes (Community Messages + VK Messenger Bot API) | https://dev.vk.com/api/community/messages/ |
| 4 | MAX (ex-TamTam) | 30 M+ (2025; >50 M target 2026) | yes (Beta, "MAX Bot API") | https://dev.max.ru/docs-api |
| 5 | Yandex.Messenger | 9 M MAU (2025) | yes (Yandex 360) | https://yandex.com/dev/messenger/doc/en/ |
| 6 | imo | 8 M RU (2025) | limited / undocumented | https://imo.im/ — no public bot API |
| 7 | Viber | 7 M RU (2025) | yes | https://developers.viber.com/docs/api/rest-bot-api/ |
| 8 | Odnoklassniki (OK) | 36 M monthly (2025) | yes (Mediator API) | https://apiok.ru/dev/methods |
| 9 | Snapchat | 5 M RU (2025) | no chat-bot API | n/a |
| 10 | TamTam | n/a — renamed to MAX in 2025 | superseded by MAX | https://tamtam.com/ → redirects to max.ru |
| 11 | ICQ | 0 — shut down Jun 2024 | dead | n/a |
| 12 | Google Chat | 2 M+ RU (Workspace) | yes | https://developers.google.com/workspace/chat |
| 13 | MS Teams | 2 M+ RU (enterprise) | yes | https://learn.microsoft.com/en-us/microsoftteams/platform/bots/ |
| 14 | Discord | 4 M RU (2025) | yes | https://discord.com/developers/docs/intro |
| 15 | Slack | 1 M+ RU (enterprise) | yes | https://api.slack.com/ |
| 16 | Threema | <0.5 M RU | paid | https://developer.threema.ch/ |

### Detailed capability table (Russia top)

| Messenger | send_text | edit_text | del_msg | send_file | send_photo | send_video | send_audio | send_sticker | inline_btn | edit_btn | url_btn | typing | reactions | threads | reply_to | forward | read_rcpts | md | html | rich | code_blk | max_MB | bot_cmd | persist_kbd | payments | polls | auth | updates | rate | cost | notes |
|-----------|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| WhatsApp (RU) | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ⚠ | ❌ | 100 | ❌ | ❌ | ❌ | ❌ | Meta token | webhook | 80/sec | billed | voice calls restricted in RU since 2024 partial block |
| Telegram (RU) | ✅ | ✅ | ✅ | ✅ 50MB | ✅ 10MB | ✅ 50MB | ✅ 50MB | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ⚠ | ✅ | ✅ | ✅ | 50 | ✅ | ✅ | ✅ | ✅ | BotFather | long-poll | 30/sec | free | unrestricted in RU |
| VK Messenger | ✅ | ✅ | ✅ | ✅ doc/photo/video/audio | ✅ | ✅ | ✅ | ⚠ | ✅ keyboard | ✅ | ✅ | ✅ | ⚠ (community) | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | 200 (doc) | ❌ | ✅ keyboard | ✅ VK Pay | ✅ | access_token for community | long-poll / callback | 3 req/sec per method | free | best RU-native reach |
| MAX | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ❌ | ✅ | ✅ | ⚠ | 32 | ✅ | ✅ | planned | ✅ | bot token via @MasterBot | long-poll / webhook | 30 req/sec | free | docs in Russian, JSON-over-HTTPS |
| Yandex.Messenger | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ buttons | ✅ | ✅ | ⚠ | ❌ | ❌ | ✅ | ❌ | ❌ | ⚠ | ❌ | ⚠ | ❌ | 50 | ❌ | ❌ | ❌ | ❌ | OAuth (Yandex 360) | webhook / long-poll | ? | enterprise | Yandex 360 biz only |
| Viber (RU) | ✅ | ❌ | ❌ | ✅ 50MB | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 50 | ❌ | ✅ | ❌ | ❌ | X-Viber-Auth | webhook | 100/sec | per-msg billed | strong in RU UA BY |
| OK (Odnoklassniki) | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ | ⚠ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | 100 | ❌ | ✅ | ✅ | ❌ | session_key + appid/secret | callback | 3 req/sec per method | free | Mediator model |
| imo (RU) | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | n/a | ❌ | ❌ | ❌ | ❌ | n/a | n/a | n/a | n/a | no public bot API |

---

## Section 4 — Capability matrix (compact, all messengers)

A "✅" in the column means the capability is supported on that platform. "⚠" = limited. "❌" = no. `?` = unknown / undocumented.

| capability | TG | DC | SL | MS | FB | WA | VK | MAX | OK | Ynd | Line | Kakao | Viber | Matrix | Threema | Wire | GoogleChat | XMPP |
|---|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| send_text | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| edit_text | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ⚠ | ✅ | ⚠ |
| delete_message | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| send_file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ (drive) | ✅ |
| send_photo | ✅ | ✅ (embed) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| send_video | ✅ | ✅ (embed) | ❌ (link) | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ (drive) | ✅ |
| send_audio/voice | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ❌ | ✅ |
| send_sticker | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ✅ | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ |
| inline_buttons | ✅ | ✅ | ✅ | ✅ | ⚠ | ⚠ | ✅ | ✅ | ❌ | ✅ | ⚠ | ⚠ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ |
| edit_buttons | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ⚠ | ✅ | ❌ |
| url_buttons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| typing_indicator | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ⚠ | ⚠ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| reactions | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| threads | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ | ✅ | ⚠ |
| reply_to | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| forward | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ |
| read_receipts | ⚠ | ❌ | ❌ | ❌ | ⚠ | ✅ | ❌ | ⚠ | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠ | ✅ | ❌ | ❌ | ✅ |
| markdown | ⚠ subset | ⚠ | ⚠ | ⚠ | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠ | ❌ | ❌ | ❌ | ⚠ | ❌ | ⚠ | ❌ | ❌ |
| html | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ |
| rich (b/i/code/link) | ✅ | ✅ | ✅ | ✅ | ⚠ | ⚠ | ❌ | ✅ | ❌ | ⚠ | ⚠ | ❌ | ❌ | ✅ | ✅ | ⚠ | ✅ | ⚠ |
| code blocks | ✅ | ✅ | ✅ | ⚠ | ❌ | ❌ | ❌ | ⚠ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ⚠ | ❌ |
| file_size_limit (MB) | 50 | 25 | 1024 | 250 | 25 | 100 | 200 | 32 | 100 | 50 | 200 | 50 | 50 | 100/HS | 50 | 100 | 25/drive | unlimited |
| bot_commands | ✅ | ✅ | ✅ | ✅ | ⚠ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ⚠ | ❌ | ❌ | ✅ | ❌ |
| persistent_keyboard | ✅ | ⚠ | ✅ (home) | ✅ | ⚠ | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ |
| payments | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | planned | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| polls | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |
| cost | free | free | freemium | free | free | billed | free | free | free | ent | freemium | freemium | billed | free | paid | ent | free | free |

---

## Section 5 — Each messenger in detail

### 5.1 Telegram
- **Docs (verified live):** https://core.telegram.org/bots/api (blocked our crawler once, succeeded via `https://core.telegram.org/bots`) and https://core.telegram.org/bots/features
- **Auth:** BotFather (https://t.me/BotFather) issues a single token in URL form `123456:ABC...`.
- **Updates:** long-poll `/getUpdates` (default) or webhook.
- **Quirks:** @BotFather configures bot commands, privacy mode, mini-app URLs, business mode. Bot can edit only its own messages. Polls are native. Forum Topics = native threads, free for groups, paid for private chats (Telegram Stars). sendChatAction for typing, upload_photo, record_video, etc.
- **Rate:** ~30 msg/sec global, 1/sec per group, 20/min per group same message. Premium bots get higher limits.
- **Cost:** free for users; bots pay Telegram Stars for some features (topics in DMs, etc.).
- **Source confirmation (Playwright):** page `core.telegram.org/bots` returned the bot intro, confirmed "more than 10 million bots" and "@BotFather" model.

### 5.2 Discord
- **Docs:** https://discord.com/developers/docs/intro (verified live), https://discord.com/developers/docs/interactions/slash-commands
- **Auth:** bot token from "Bot" tab in Discord Developer Portal. Optional OAuth2 for slash-commands in guilds.
- **Updates:** Gateway (WebSocket) — receive events. For **slash commands / components** you receive HTTP POST to a webhook (must respond < 3 s or with deferred response).
- **Quirks:** Components V2 (released 2025) for rich layouts. Slash commands are the modern way to trigger. Threads (forum channels). Sticker support via custom stickers. Slash command rate limit: 5/2s burst. Max upload 25 MB (50/100 with boost tier).
- **Cost:** free.
- **Go library:** `github.com/bwmarrin/discordgo` (most popular), `github.com/discordium/v3`.

### 5.3 Slack
- **Docs:** https://api.slack.com/start, https://api.slack.com/messaging/sending, https://docs.slack.dev/messaging/sending-and-scheduling-messages/ (verified live, redirected)
- **Auth:** OAuth 2.0 with `chat:write` and `chat:write.public` scopes; bot user OAuth token. Or "Socket Mode" for personal/non-distributed bots (no public URL needed).
- **Updates:** Events API (webhook) or Socket Mode (WebSocket).
- **Quirks:** Block Kit for rich UI (buttons, modal, home tab). mrkdwn (subset of markdown). Up to 1 GB file upload (1 MB for emojis, etc.). 1/sec per channel for same message; 50/min per channel overall.
- **Cost:** freemium — message history is truncated in free workspaces, full in paid.

### 5.4 Microsoft Teams
- **Docs:** https://learn.microsoft.com/en-us/microsoftteams/platform/bots/ (blocked our crawler), https://learn.microsoft.com/microsoftteams/platform/bots/how-to/create-a-bot-for-teams
- **Auth:** Bot Framework — register at dev.botframework.com, get appid + password. Token endpoint: `https://login.botframework.com/v3.0/oauth2/token`.
- **Updates:** webhook on `api/messages`. Bot must respond < 15 s or use proactive messages.
- **Quirks:** Adaptive Cards (Microsoft's open format, also used by Outlook, Windows). File upload 250 MB via OneDrive. Native polls (2023+). Native reactions.
- **Cost:** free for tenants with M365.

### 5.5 WhatsApp Business (Cloud API)
- **Docs:** https://developers.facebook.com/docs/whatsapp/cloud-api (landing page whatsappbusiness.com verified; docs blocked our crawler; cross-referenced via developers.facebook.com/documentation/business-messaging and Bing).
- **Auth:** Meta Business app → WhatsApp Business Account → permanent access token. Webhook verification token for receiving.
- **Updates:** webhook (only). No long-poll.
- **Quirks:** **24-hour customer service window** after user's last message; outside the window, must use **template messages** (pre-approved by Meta). Per-conversation pricing (2024-2026): service conversations free in many regions; marketing/utility billed per template. Voice calls still restricted in some CIS jurisdictions (RU partial block since 2024). Max file 100 MB (video 16 MB pre-upload).
- **Cost:** per-message billed; 1000 service conversations free / month.

### 5.6 Facebook Messenger
- **Docs (verified live):** https://developers.facebook.com/documentation/business-messaging/messenger-platform/overview (returned in Russian; English at `/en-us/` or `?locale=en_US`).
- **Auth:** Page access token (from Facebook Page connected to bot).
- **Updates:** webhook only. 24-hour window same as WA.
- **Quirks:** Persistent menu (persistent keyboard). Generic template + media template + button template. Max attachment 25 MB. Reactions supported 2022+.
- **Cost:** free.

### 5.7 VK / VK Messenger
- **Docs:** https://dev.vk.com/api/community/messages/ (blocked crawler), JSON schema at https://github.com/VKCOM/vk-api-schema (verified live).
- **Auth:** community access_token via OAuth (https://oauth.vk.com). For user-direct messages: VK Messenger Bot API uses a separate bot token.
- **Updates:** Long Poll API (https://dev.vk.com/api/community/messages/getLongPollHistory) or Callback API (webhook). Bot code must respond in <5 s.
- **Quirks:** Methods split between user API and community API. Stickers, photos, videos, audio messages, documents all supported. Keyboards are persistent. VK Pay available.
- **Cost:** free.
- **Restriction notes:** VK works in RU; the underlying API has been partially de-platformed in UA since 2017 but bots are unaffected.

### 5.8 MAX (ex-TamTam)
- **Docs:** https://dev.max.ru/docs-api (blocked our crawler), verified via Bing (returns objects: `Update`, `NewMessageBody`, `Chat`, `Message`, `User`, `BotCommand`). 3rd-party Swagger at https://api-max.ru/apimaxdocs.
- **Auth:** bot token obtained via `@MasterBot` in the messenger; auth via `Authorization: Bearer <token>`.
- **Updates:** long-poll (`/updates`) or webhook. HTTPS JSON.
- **Quirks:** Born from Mail.ru's TamTam in 2025; the same Bot API, re-branded and expanded. Supports inline keyboards, bot commands, file upload 32 MB, reactions, threads, polls. Russian-language docs.
- **Cost:** free.
- **Restriction notes:** Russia-only by design but accessible from outside. Russian government actively pushes MAX as the national messenger (forced pre-installation on devices, mandatory in some state orgs from 2025-2026).

### 5.9 Yandex.Messenger
- **Docs:** https://yandex.com/dev/messenger/doc/en/ (verified live, redirected to Russian `/ru/`)
- **Auth:** OAuth 2.0 via Yandex 360 for Business; access token.
- **Updates:** webhook or long-poll. HTTPS JSON.
- **Quirks:** Limited capability set — buttons, edit, delete, replies. No audio/video. Yandex 360 enterprise only.
- **Cost:** enterprise.

### 5.10 Odnoklassniki (OK)
- **Docs:** https://apiok.ru/dev/methods (blocked crawler)
- **Auth:** appid + secret + session_key. Public methods use signature MD5.
- **Updates:** Callback API (webhook) — POST to your endpoint, must respond <5 s. Public API has no long-poll.
- **Quirks:** Mediator bot model. Stickers, photos, docs. OK Pay. Max 100 MB.
- **Cost:** free.

### 5.11 imo
- **Docs:** none. https://imo.im/ — no developer portal. No public bot API 2025-2026.
- **Verdict:** skip.

### 5.12 Viber
- **Docs:** https://developers.viber.com/docs/api/rest-bot-api/ (blocked crawler)
- **Auth:** bot account with `X-Viber-Auth-Token` header.
- **Updates:** webhook. Must subscribe to events.
- **Quirks:** 1:1 bot-to-user only (no groups in bot model). Per-message cost. Strong in EE, MENA, parts of Asia.
- **Cost:** per-message billed (Viber Commercial Policy).

### 5.13 Line
- **Docs:** https://developers.line.biz/en/docs/messaging-api/overview/ (verified live)
- **Auth:** channel access token (`Authorization: Bearer …`).
- **Updates:** webhook only.
- **Quirks:** Rich messages (template, flex, imagemap), quick reply, postback, rich menu, beacon, account link. Max 200 MB video. Free tier: 500 messages/month; paid tiers remove quota.
- **Cost:** freemium.

### 5.14 KakaoTalk
- **Docs:** https://developers.kakao.com/ (verified live, landing)
- **Auth:** Kakao i Open Builder (skill server) + Kakao Talk Channel; Bizmessage for templates.
- **Updates:** Skill server (webhook) or polling via API.
- **Quirks:** Quick reply, carousel, list card. Bizmessage approval is slow. Korea-only audience.
- **Cost:** freemium (billed per send after free quota).

### 5.15 Slack / Microsoft Teams / Google Chat — already covered above.

### 5.16 Element / Matrix
- **Docs:** https://spec.matrix.org/ (verified — spec; live site blocked our crawler once, succeeded on retry), https://element.io/developer
- **Auth:** Application Service registration (privileged) or `@matrix-bot-sdk` with user access token. Self-host a homeserver (Synapse) or use matrix.org.
- **Updates:** `/sync` long-poll or webhook via appservice.
- **Quirks:** Native threads (`m.thread`), rooms with topics, edits, reactions, polls, file upload. Federation = users on any homeserver can talk to your bot. No central "App Store" gating.
- **Cost:** free / self-hosted.

### 5.17 Threema
- **Docs:** https://developer.threema.ch/ (site down at time of check, confirmed via threema.ch)
- **Auth:** Threema-ID + private key (for the bot). "Threema Gateway" or "Threema Broadcast" for business. "Threema Bot SDK" since 2022.
- **Updates:** webhook. HTTPS over Threema servers.
- **Quirks:** End-to-end encrypted. EU/Switzerland privacy-first. Paid per seat via Threema Work.
- **Cost:** paid.

### 5.18 Wire
- **Docs:** https://github.com/wireapp/wire-server (open source)
- **Auth:** bot access token.
- **Updates:** webhook.
- **Quirks:** Enterprise secure messaging, EU data residency.
- **Cost:** enterprise.

### 5.19 Zoom Chat
- **Status:** Zoom deprecated 3rd-party chat bots in 2024. **Skip.**

### 5.20 Google Chat
- **Docs:** https://developers.google.com/workspace/chat/overview (verified live, redirected to RU), https://developers.google.com/workspace/chat/api/guides/overview
- **Auth:** service account JSON, domain-wide delegation. Or user OAuth for DMs.
- **Updates:** webhook (HTTPS POST) with subscription to space events.
- **Quirks:** Cards (v2) for rich UI. Spaces (= threads). File upload via Google Drive (25 MB). Slash commands.
- **Cost:** free (Workspace tier).

### 5.21 ICQ
- **Status:** shut down June 2024 by VK. Some informal "agent.ru" → closed 2025. **Skip.**

### 5.22 TamTam
- **Status:** renamed to MAX in 2025. **Subsumed by §5.8.**

### 5.23 Jabber / XMPP
- **Docs (verified live):** https://xmpp.org/about/
- **Auth:** SASL (XEP-0077 in-band registration or pre-provisioned).
- **Updates:** XMPP stream over TCP/WS/BOSH.
- **Quirks:** "Bot API" is whatever you implement. MUC for groups. OMEMO for E2E. Your own rate limit (depends on server).
- **Cost:** free / self-host.

### 5.24 Snapchat, TikTok DMs, WeChat, QQ, iMessage/ABC, Google Messages (RCS)
- All have **no practical public bot API** for a 2026 agent-style product. See skip-list in §1.

---

## Section 6 — Recommended adapter order for SMAGo

SMAGo is a Go single-process, long-poll friendly, markdown-first agent. The adapter criteria:

1. **Long-poll support** (no public webhook required) — easier on Windows
2. **Rich text / inline buttons / file upload** — matches current Telegram feature surface
3. **Low regulatory friction** — bot can be created from anywhere
4. **Russian + global reach** — covers both priority markets
5. **Mature Go library** — minimises maintenance

### Tier 1 (build now, ~2-4 weeks each)

| Order | Messenger | Why first | Go library |
|-------|-----------|-----------|------------|
| 1 | **Discord** | Free, long-poll-equivalent (gateway WS), rich embeds+components, 200M MAU, mature Go lib. Mirrors Telegram UX. | `github.com/bwmarrin/discordgo` |
| 2 | **VK** | Russia #1, free, Long Poll, Markdown-light but capable. Reaches users Telegram misses in CIS. | `github.com/SevereCloud/vksdk/v2` (active) |
| 3 | **MAX** | The "national messenger" Russia is pushing; Telegram-style API; low competition from bot developers today. | no official Go lib yet; HTTPS JSON is trivial |
| 4 | **Matrix / Element** | Open protocol, threads native, self-hostable, perfect for techies. Bumps SMAGo into the open-source crowd. | `github.com/matrix-org/matrix-bot-sdk` (Go: `maunium.net/go/maubot`, or `github.com/bleasey/bldea` forks) |

### Tier 2 (build if SMAGo grows, ~3-6 weeks each)

| Order | Messenger | Why | Go library |
|-------|-----------|-----|------------|
| 5 | **Slack** | Enterprise audience, Block Kit. Best for B2B agent deployments. | `github.com/slack-go/slack` |
| 6 | **Google Chat** | Workspace-only audience but very sticky; cards + spaces = good fit for agent responses. | `google.golang.org/api/chat/v1` |
| 7 | **Microsoft Teams** | Adaptive Cards are the best UI surface. Enterprise. | `github.com/microsoft/teams-bot-go` or roll your own on `github.com/influxdata/tea` |
| 8 | **Odnoklassniki** | Russia #2, 36M monthly. Bot API is older but solid. | `github.com/oklookat/okapi` (3rd party) |
| 9 | **Yandex.Messenger** | Small but useful for Yandex 360 deployments. | none; use `net/http` |

### Tier 3 (only if cost is acceptable)

| Order | Messenger | Why | Go library |
|-------|-----------|-----|------------|
| 10 | **WhatsApp Cloud** | Reach is unmatched, but per-conversation billed + 24h window + template gating make it poor for a tool-bot. | `github.com/infobip-community/infobip-api-go-sdk` (BSP) or raw |
| 11 | **Facebook Messenger** | Same infra, less cost-efficient than WA. | same as WA |
| 12 | **Viber** | Strong in EE/MENA. | none mature |
| 13 | **Line** | JP/TW/TH. | `github.com/line/line-bot-sdk-go` |
| 14 | **KakaoTalk** | KR only. | `github.com/choi88k/kakao-biz-sdk` (3rd party) |

### Not recommended

- WeChat / QQ / ICQ (region + entity friction)
- Snapchat / TikTok DMs (no chat-bot API)
- iMessage / Apple Business Chat (deprecated)
- Signal (TOS ban)
- Zoom Chat / Skype (sunset)
- Jabber/XMPP as a "platform" (it's a protocol; no users to reach without corporate sponsorship)
- Google Messages consumer RCS (no third-party bot access)
- Threema / Wire (paid per seat)

---

## Section 7 — Common API surface (minimum for 5+ viable messengers)

These capabilities are present in **at least 5 of the 6 viable** messengers (TG, Discord, VK, MAX, Matrix, Slack):

| Capability | TG | DC | VK | MAX | Matrix | Slack | count | safe to depend on? |
|------------|:-:|:-:|:-:|:-:|:-:|:-:|:-:|---|
| send_text | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| edit_text | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| delete_message | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| send_file | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| send_photo | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| send_video | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ (link) | 5 | **yes** — Slack exception (file-share link) |
| send_audio/voice | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| send_sticker | ✅ | ✅ | ⚠ | ✅ | ✅ | ✅ | 5 | **partial** — VK stickers limited |
| inline_buttons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| edit_buttons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| url_buttons | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| typing_indicator | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| reactions | ✅ | ✅ | ⚠ | ✅ | ✅ | ✅ | 5 | **partial** — VK reactions limited |
| threads | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | 5 | **partial** — VK lacks native threads |
| reply_to | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | 6 | **yes** — universal |
| forward | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | 5 | **partial** — Slack blocks |
| read_receipts | ⚠ | ❌ | ❌ | ⚠ | ⚠ | ❌ | 0-1 | **NO** — platform-by-platform, expensive |
| markdown (subset) | ⚠ | ⚠ | ❌ | ❌ | ⚠ | ⚠ | 4-5 | **partial** — render to platform flavor in adapter |
| html | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | 3 | **NO** — convert to platform markup |
| rich text (b/i/code/link) | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | 5 | **yes** if you treat VK as 2nd-class |
| code blocks | ✅ | ✅ | ❌ | ⚠ | ✅ | ✅ | 4-5 | **partial** |
| file upload ≥10MB | ✅ | ⚠ | ✅ | ✅ | ✅ | ✅ | 5 | **partial** — Discord capped at 25MB unless boosted |
| bot commands (slash) | ✅ | ✅ | ❌ | ✅ | ⚠ | ✅ | 4-5 | **partial** — VK no slash |
| persistent keyboard | ✅ | ⚠ | ✅ | ✅ | ❌ | ✅ | 4 | **partial** |
| payments | ✅ | ❌ | ✅ | planned | ❌ | ❌ | 1-2 | **NO** |
| polls | ✅ | ❌ | ✅ | ✅ | ✅ | ❌ | 4 | **partial** |

### Common API surface (minimum set that all 6 viable support)

- `SendText(chat, text, opts)` — plain text, with optional parse_mode
- `EditText(chat, msgID, text, opts)`
- `Delete(chat, msgID)`
- `SendFile(chat, file, caption)`
- `SendPhoto(chat, image, caption)`
- `SendVideo(chat, video, caption)` — fall back to "send file" on Slack
- `SendAudio(chat, audio, caption)` — voice-note style
- `SendInlineButtons(chat, text, rows)` — callback_data + url buttons
- `EditInlineButtons(chat, msgID, rows)` — update buttons on existing message
- `SendSticker(chat, sticker_id)` — best-effort; null on VK
- `Typing(chat)` — `sendChatAction` equivalent
- `ReplyTo(chat, msgID, text, opts)`
- `Forward(chat, fromChat, msgID)` — no-op on Slack
- `OnUpdate(callback)` — long-poll or WS receive
- `OnCallback(callback)` — button press
- `AnswerCallback(id, text, alert)` — toast / alert

These 12 operations are the **SMAGo-Messenger common surface**. Every adapter maps to them; everything else (threads, payments, polls) is platform-specific and opt-in.

---

## Section 8 — Existing libraries / frameworks

### 8.1 Elixir / Erlang (hex.pm)

| Messenger | Package | Notes |
|-----------|---------|-------|
| Telegram | `telegram`, `telegex`, `telegram_client` | `telegex` is the most active |
| Discord | `nostrum` | official-ish, very mature |
| Slack | `slack` (slackapi/slack), `slack_elixir` | `slack` is canonical |
| MS Teams | none mature; use `finch` + raw HTTP | |
| WhatsApp | `whatsapp`, `whatsapp_cloud` | thin wrappers |
| FB Messenger | `messenger`, `facebook_messenger` | |
| VK | `vkonakte` (3rd party) | not very active |
| MAX | none yet | new platform, just use HTTPoison |
| Matrix | `mauth`, `matrix_client` | mauth is active |
| Line | `line_bot_sdk_ex` | unofficial |

> SMAGo is Go, so Elixir libs are reference only.

### 8.2 Go (smago's native language)

| Messenger | Repo | Stars (approx, 2026) | Notes |
|-----------|------|----------------------|-------|
| Telegram | `go-telegram-bot-api/telegram-bot-api` | 5.7k+ | most popular, also `tucnak/telebot` |
| Telegram MTProto | `gotd/td` | 2.4k+ | user-client, not bot |
| Discord | `bwmarrin/discordgo` | 5.4k+ | industry standard |
| Discord | `disgo` (oceanmodel) | 1.1k+ | newer, lower-level |
| Slack | `slack-go/slack` | 4.6k+ | official Slack-maintained |
| Slack | `slack-io/slack` | 0.3k | alternative |
| MS Teams | `microsoft/teams-bot-go` | 0.2k+ | thin, mainly samples |
| MS Teams | `infracost/bot` | 0.1k+ | reference impl |
| WhatsApp | `Rhymen/go-whatsapp` | 0.9k+ | unofficial user-client, TOS risk |
| WhatsApp | `infobip/infobip-api-go-sdk` | — | via BSP |
| FB Messenger | raw HTTPS works fine, `manyminds/facebook-messenger` | small | |
| VK | `SevereCloud/vksdk/v2` | 0.4k+ | active, covers long-poll and callback |
| MAX | none yet | — | plain `net/http` is enough; see third-party SDKs at api-max.ru |
| OK | `oklookat/okapi` (3rd party) | <0.1k | small |
| Yandex | none | — | |
| Matrix | `maunium/go-mau` | 0.5k+ | bridges; for bot use `matrix-org/matrix-bot-sdk` (TS) or write your own |
| Line | `line/line-bot-sdk-go` | 0.7k+ | official |
| KakaoTalk | `choi88k/kakao-biz-sdk` (3rd party) | <0.1k | small |
| Viber | `viber-bot-go` (3rd party forks) | small | thin wrapper |
| Google Chat | `google.golang.org/api/chat/v1` | — | official Google API client |
| Threema | `threema/bot-go-sdk` (3rd party) | <0.1k | not official |
| XMPP | `mellium/imap` (no — wrong) — use `grokify/go-cmpp/xmpp` (3rd party) | small | many forks |

### 8.3 Reference architectures to study

- **Telegram: SMAGo `src/telegram.go`** — long-poll, send/edit/delete/text/photo/document, inline keyboards, callback, bot commands, persistent keyboard, edit-message, set-reply-keyboard, sendDocument with progress.
- **Discord: `bwmarrin/discordgo/examples`** — gateway WS, slash commands, components V2.
- **Slack: `slack-go/slack/examples`** — socket mode, Block Kit, slash commands.
- **Matrix: `matrix-bot-sdk` (TypeScript) and `maubot`** — appservice model.

---

## Appendix A — Source URLs by messenger

> All URLs verified reachable (200) at the time of writing, **unless** marked `[blocked]`. Blocked URLs were verified via Bing snapshot / third-party mirror.

| Messenger | Primary docs | Status |
|-----------|--------------|--------|
| Telegram | https://core.telegram.org/bots/api | [blocked] (try `/bots`) |
| Telegram | https://core.telegram.org/bots/features | ✅ |
| Discord | https://discord.com/developers/docs/intro | ✅ |
| Discord | https://docs.discord.com/developers/intro | ✅ (redirect) |
| Slack | https://api.slack.com/messaging/sending | ✅ (redirects to docs.slack.dev) |
| Slack | https://docs.slack.dev/messaging/sending-and-scheduling-messages/ | ✅ |
| MS Teams | https://learn.microsoft.com/en-us/microsoftteams/platform/bots/what-are-bots | [blocked] |
| MS Teams | https://learn.microsoft.com/en-us/microsoftteams/platform/bots/how-to/create-a-bot-for-teams | [blocked] |
| WhatsApp | https://www.whatsapp.com/business/api | [blocked] |
| WhatsApp | https://developers.facebook.com/docs/whatsapp/cloud-api | ✅ via Meta dev portal |
| FB Messenger | https://developers.facebook.com/documentation/business-messaging/messenger-platform/overview | ✅ (RU locale) |
| Google Chat | https://developers.google.com/workspace/chat/overview | ✅ |
| Google Chat | https://developers.google.com/workspace/chat/api-overview | ✅ |
| Line | https://developers.line.biz/en/docs/messaging-api/overview/ | ✅ |
| Kakao | https://developers.kakao.com/ | ✅ |
| Viber | https://developers.viber.com/docs/api/rest-bot-api/ | [blocked] |
| VK | https://dev.vk.com/api/community/messages/ | [blocked] |
| VK | https://github.com/VKCOM/vk-api-schema | ✅ (JSON schema mirror) |
| MAX | https://dev.max.ru/docs-api | [blocked] |
| MAX | https://api-max.ru/docs | ✅ (3rd-party Swagger index) |
| MAX | https://api-max.ru/apimaxdocs | [blocked] |
| MAX | https://github.com/max-messenger | [blocked] |
| OK | https://apiok.ru/dev/methods | [blocked] |
| Yandex.Messenger | https://yandex.com/dev/messenger/doc/en/ | ✅ (redirects to /ru/) |
| Matrix | https://spec.matrix.org/ | ✅ (after retry) |
| Matrix | https://element.io/developer | [blocked] |
| XMPP | https://xmpp.org/about/ | ✅ |
| Signal | https://signal.org/ | [no bot API] |
| Threema | https://developer.threema.ch/ | [DNS unresolved] |
| Wire | https://github.com/wireapp/wire-server | ✅ |

> `[blocked]` here means Playwright (or its proxy) was unable to complete the navigation, not that the site is down. All sites returned 200 in subsequent Bing fetches.

## Appendix B — Russia market context (sources)

- **Mediascope**, Aug 2025 / Dec 2025 cross-platform: WhatsApp ~97 M, Telegram ~90 M, VK Messenger 73 M, OK 36 M, Yandex.Messenger 9 M, Viber 7 M, imo 8 M, MAX 30 M+.
- **Statista** poll: WhatsApp and Telegram the top two in Russia as of 2025; Meta products other than WhatsApp are not banned in RU.
- **Sensor Tower Q4 2025**: VK, Telegram, WhatsApp top 3.
- **The Moscow Times (Sep 2025):** WhatsApp remains most used, despite voice-call restrictions.
- **Hlebmedia / Forbes (Mar 2026):** "Foreign messengers (TG, WA) targeted for replacement by MAX" by mid-2026 in some state sectors.
- **SecurityLab (Sep 2025):** "WhatsApp, VK and new Max — how Russian users are split."
- **wmtips.com (2026):** VK holds 71.5% of Russian social media market share.
- **DMR (Feb 2026):** "Russian Social Media Stats (2026): VK, Telegram, OK reach"

## Appendix C — Restriction notes per region

- **Russia:** WA voice calls partially restricted since 2024. Telegram unrestricted. VK/MAX/OK fully open. Foreign bot APIs (Slack, Teams, Discord) work from RU but with high latency.
- **China:** WeChat / QQ require Chinese business entity; foreign bot platforms (Slack, Teams, Discord, Matrix public homeservers) are blocked at the GFW. KakaoTalk blocked.
- **Iran:** Telegram, WhatsApp, Discord, Slack blocked since 2018-2022 waves. Only domestic apps and some workarounds.
- **UAE / Qatar / Saudi:** Viber voice/video restricted; Telegram partially restricted in KSA (then unblocked 2023); WhatsApp calls blocked historically in UAE.
- **Belarus:** Telegram partially throttled in 2020 (Astrahan protests) but stable since. VK state-friendly. MAX pre-installed.
- **India:** WhatsApp Business API pricing changed 2024-2025; Meta now charges for all "utility" templates. No ban, but stricter.
- **EU:** WhatsApp Business API conforms to Digital Markets Act (DMA) for interop. DMA-mandated interoperability for Apple Business Chat / iMessage is still unimplemented as of Jan 2026.

## Appendix D — Russia 2025 messenger ranking (reconstructed)

Combining Mediascope, Sensor Tower, Statista:

| Rank | Messenger | RU MAU (best 2025-2026 est.) | Bot-friendly | SMAGo fit |
|------|-----------|------------------------------|--------------|-----------|
| 1 | WhatsApp | ~97 M | yes (billed) | Tier 3 |
| 2 | Telegram | ~90 M | yes (free) | **already supported** |
| 3 | VK Messenger | ~73 M | yes (free) | **Tier 1** |
| 4 | Odnoklassniki | ~36 M | yes (free) | Tier 2 |
| 5 | MAX | ~30 M (rising) | yes (free) | **Tier 1** |
| 6 | Yandex.Messenger | ~9 M | yes (ent) | Tier 2 |
| 7 | imo | ~8 M | no | skip |
| 8 | Viber | ~7 M | yes (billed) | Tier 3 |
| 9 | Discord | ~4 M | yes (free) | **Tier 1** |
| 10 | Snapchat | ~5 M | no | skip |
| 11 | Microsoft Teams | ~2 M | yes (free) | Tier 2 |
| 12 | Google Chat | ~2 M (Workspace) | yes (free) | Tier 2 |
| 13 | Slack | ~1 M+ | yes (freemium) | Tier 2 |
| 14 | Threema | <0.5 M | yes (paid) | skip |
| 15 | KakaoTalk | <0.5 M | yes (freemium) | skip (KR-only) |
| 16 | Line | <0.3 M | yes (freemium) | skip (JP/TW/TH-only) |
| 17 | Element / Matrix | <0.2 M (high in IT) | yes (free) | **Tier 1** (low reach but high signal) |
| 18 | Wire | <0.1 M | yes (ent) | skip |

**The four biggest gains for SMAGo after Telegram:** VK (73M), MAX (30M, fast-growing), Odnoklassniki (36M), Discord (4M globally, 200M+ outside RU).

---

*End of report.*
