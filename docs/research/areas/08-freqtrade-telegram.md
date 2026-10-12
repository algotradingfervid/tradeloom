# 08 — freqtrade Telegram bot: what it does, and what Tradeloom should take from it

Researched 2026-10-12. This document uses freqtrade only as **design inspiration**. freqtrade is GPL-3.0. Everything below is a paraphrase of behaviour and feature design. No code or verbatim text is copied, apart from short identifiers (command names, config keys, enum values), which are needed to cite facts.

**Primary sources**

- Docs: [freqtrade.io/en/stable/telegram-usage](https://www.freqtrade.io/en/stable/telegram-usage/). The repository copy `docs/telegram-usage.md` at the same commit as the source was read.
- Source: shallow clone of [github.com/freqtrade/freqtrade](https://github.com/freqtrade/freqtrade) at commit `d636f8e4` (2026-10-11). Links below are pinned to that commit (`FT/…` = `https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/…`).
  - `freqtrade/rpc/telegram.py` (2,336 lines)
  - `freqtrade/rpc/rpc.py`, `rpc_manager.py`, `rpc_types.py`
  - `freqtrade/enums/rpcmessagetype.py`
  - `freqtrade/config_schema/config_schema.py`
  - `freqtrade/data/dataprovider.py`
  - `requirements.txt`, which pins `python-telegram-bot==22.8`
- Telegram: [Bot API](https://core.telegram.org/bots/api) (Bot API 10.3, changelog dated 2026-08-24) and [Bots FAQ](https://core.telegram.org/bots/faq).
- Go libraries: [pkg.go.dev](https://pkg.go.dev), proxy.golang.org `@latest` and the GitHub release API (§E).

Labels: **OBSERVED-IN-CODE** means it was read in source at the commit above. **DOCUMENTED** means it was read in the docs. **UNVERIFIED** means I could not confirm it from a primary source.

---

## A. Setup and security (freqtrade)

### A1. Configuration keys (`telegram` block)

These come from the config schema ([FT/freqtrade/config_schema/config_schema.py#L582](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/config_schema/config_schema.py#L582)).

| Key | Type / default | Purpose |
|---|---|---|
| `enabled` | bool, required | Turns the Telegram module on |
| `token` | string, required | Bot token issued by @BotFather |
| `chat_id` | string, required | The one chat (private user id, or a group id with a leading `-`) the bot talks to and accepts commands from. Can also come from the env var `FREQTRADE__TELEGRAM__CHAT_ID` |
| `topic_id` | string, optional | A forum-topic (thread) id inside a group. Replies and notifications go to that thread |
| `authorized_users` | string[], optional | User-id allowlist for **control**. Absent means anyone in the chat may command. Present but empty means nobody may command; members only receive messages |
| `allow_custom_messages` | bool, default true | Master switch for strategy-originated messages |
| `balance_dust_level` | number ≥ 0 | Threshold under which `/balance` lumps small coin balances into one "other" line |
| `notification_settings` | object | Per-message-type loudness (§C1) |
| `reload` | bool (default effectively true) | Whether certain replies get an inline **Refresh** button |
| `keyboard` | string[][] | Custom reply keyboard (§C4). Documented, and validated in code ([FT/…/telegram.py#L173](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L173)) |

### A2. Onboarding flow (DOCUMENTED)

1. Create a bot with @BotFather (`/newbot`) and copy the token.
2. Press Start in the new bot chat. A bot cannot message a user who has never opened its chat.
3. Find the user id through a third-party "user info" bot. For groups and topics, add the bot to the group and send **`/tg_info`**. The bot replies, in the same chat, with a ready-to-paste JSON snippet: `chat_id`, `topic_id` if any, and a commented-out `authorized_users` entry.
4. **`/tg_info` is deliberately unauthenticated.** It answers in whatever chat it was sent from, even when that chat is not the configured `chat_id` (OBSERVED-IN-CODE, [#L2300](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L2300)). It reveals only ids that the asker can already see.
5. Docs warning: in a group, every member gets full control unless `authorized_users` limits it.

### A3. Authorization (OBSERVED-IN-CODE)

A decorator wraps every command handler ([#L99](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L99)). For each command or callback it checks, in order:

1. **Chat id.** The update's chat id must equal the configured `chat_id`. Otherwise the update is dropped and an INFO line logs the rejected chat id.
2. **Topic.** If `topic_id` is configured, the message's thread id must match. Otherwise it is dropped and logged at DEBUG only, because this is common in multi-topic groups.
3. **User.** If `authorized_users` is configured, the sender's user id must be in it. Otherwise it is dropped and an INFO line notes the unauthorized attempt.
4. **No reply** is ever sent to an unauthorized sender (a silent drop).
5. **Handler errors.** Business errors (`RPCException`) are sent back to the chat as plain text. Any other exception is logged with a stack trace and swallowed, so a bad command can never kill the bot. The DB session is rolled back before the handler and removed after it.

**Gap noticed while reading.** The two inline-button handlers that *execute* trades are not wrapped by the authorization decorator: force-exit picker (`_force_exit_inline`, [#L1498](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L1498)) and force-entry picker (`_force_enter_inline`, #L1537). The *Refresh* callbacks reuse decorated handlers, so they are checked. In a group with `authorized_users` set, a non-authorized member could therefore press a force-exit button that an authorized user had summoned. This is an observation from reading only; it was not tested. **Lesson for Tradeloom:** authorize every callback query exactly as strictly as commands, and bind each confirmation button to the user who requested it (FR-TG-004, FR-TG-020).

### A4. Reload behaviour

- **`/reload_config`** sets the bot state to `RELOAD_CONFIG` ([FT/…/rpc.py#L993](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/rpc.py#L993)). The worker loop then tears down and rebuilds the bot, the RPC modules included (`worker.py#L80`). Runtime-only state is lost on reload: blacklist additions, `marketdir`, and pause.
- **Polling start** ([#L352](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L352)) retries initialisation 3 times, 2 s apart. If all attempts fail it logs a warning and continues **without** Telegram; trading is not aborted. It then long-polls with a 20 s timeout and 10 bootstrap retries, and **`drop_pending_updates=True`**: any commands sent while the bot was down are discarded at startup rather than executed late.
- **"Reload throttling".** freqtrade has **no** time-based throttle on Refresh presses, as far as the code shows (UNVERIFIED that none exists elsewhere). The only controls are:
  - the global `reload` on/off switch;
  - silently ignoring Telegram's "message is not modified" error when a refresh produces identical text ([#L2163](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L2163));
  - stamping each refreshed message with an "Updated: <time>" suffix, which also makes the text differ.

---

## B. Command reference (freqtrade, exact list from code)

The registration list ([#L273–L318](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L273)) has **39 command handlers covering 51 command strings** once aliases are counted. There are also **15 inline-callback handlers** (#L319): 13 Refresh targets plus the force-exit and force-enter pickers.

**`/edge` no longer exists** in the current code. It is absent from both the handler list and the help text, so it is not documented below.

Descriptions are paraphrased.

### B1. Bot control

| Command (aliases) | Args | What it does / returns |
|---|---|---|
| `/start` | – | Sets state RUNNING. Replies with the status, or "already running" |
| `/stop` | – | Sets state STOPPED. Open trades are no longer managed |
| `/pause` (`/stopentry`, `/stopbuy`) | – | State PAUSED: no new entries, but open trades are still managed (exits, stoploss, and only reducing position adjustments). Not persisted across restart |
| `/reload_config` (`/reload_conf`) | – | Rebuilds the bot from the config file (§A4) |
| `/show_config` (`/show_conf`) | – | Summary of the running config: mode (dry or live), exchange, market type, stake, max open trades, ROI table, entry and exit pricing JSON, stoploss or trailing settings, position adjustment, timeframe, strategy, state |
| `/logs` | `[n]` (default 10) | Last *n* in-memory log records, split into several messages near the 4096-char limit |
| `/health` | – | Last loop-processing time, first-ever start time, last restart time |
| `/version` | – | Bot version, plus strategy version if the strategy declares one |
| `/help` | – | Grouped command list. Force-entry lines appear only if `force_entry_enable` is on, and short only on non-spot markets |
| `/tg_info` | – | Unauthenticated setup helper (§A2) |

### B2. Status and trades

| Command | Args | What it does / returns |
|---|---|---|
| `/status` | – \| `<id> [<id>…]` \| `table` | **No args:** one detailed message per open trade (id, age, pair, direction or leverage, amount, stake, entry tag, open, current and close rates, unrealised and realised profit, stoploss with distance, open-order info, filled-order breakdown when position adjustment is on). **With ids:** only those trades. **`table`:** compact table (id, L/S, pair, age, profit). Pending entry orders are marked `*` and pending exits `**`. Paginated at 50 rows per message. Totals row in fiat. Has a Refresh button |
| `/order` | `<id> [<id>…]` | The order list (fills) for the given trades |
| `/trades` | `[n]` (default 10) | Last *n* closed trades as a table: close age, pair (id, L/S), profit % and absolute |
| `/count` | – | Open trades vs `max_open_trades`, plus total stake. Refresh button |
| `/locks` | – | Active pair locks (id, pair, until, reason), 25 per message |
| `/unlock` (`/delete_locks`) | `<pair\|lock_id>` | Removes a lock (numeric means id, otherwise pair), then re-lists locks |
| `/list_custom_data` | `<trade_id> [key]` | Shows the strategy's per-trade key/value store entries |
| `/marketdir` | `[long\|short\|even\|none]` | Reads or sets a user-managed "market direction" variable that strategies can read. Not persisted |

### B3. Trade-modifying (dangerous)

| Command | Args | Behaviour |
|---|---|---|
| `/forceexit` (`/forcesell`, `/fx`) | `<id>` \| `all` \| none | **With an id or `all`:** executes **immediately**, with no confirmation step. **No args:** sends an inline picker listing each open trade, plus a Cancel button. Pressing a trade button executes the exit straight away and edits the message to say so. Refuses if the bot is STOPPED |
| `/forcelong` (`/forcebuy`), `/forceshort` | `<pair> [rate]` \| none | Only if `force_entry_enable` is true. With a pair it enters immediately (a rate makes it a limit order). With no args it shows an inline picker of whitelist pairs plus Cancel. Trades get the entry tag `force_entry`. Short works only on non-spot markets |
| `/delete` | `<id>` | Removes the trade from the DB, tries to cancel its open orders and stoploss-on-exchange orders, and tells the user to handle the asset on the exchange manually |
| `/cancel_open_order` (`/coo`) | `<id>` | Cancels a trade's open order(s), using the reason "user cancel". Fails if there is no open order or the bot is STOPPED |
| `/reload_trade` | `<id>` | Re-reads a trade's orders from the exchange, which helps when a trade was closed manually there. Live mode only |
| `/blacklist` | `[pair …]` | Shows the blacklist, or adds pairs or patterns to it. Runtime only |
| `/blacklist_delete` (`/bl_delete`) | `<pair …>` | Removes entries from the runtime blacklist |
| `/whitelist` | `[sorted] [baseonly]` | Shows the active tradable pair list. Read-only |

### B4. Metrics

| Command | Args | Returns |
|---|---|---|
| `/profit` | `[n]` or `long\|short [n]` | Closed-trade and all-trade ROI (absolute, %, fiat), trade count, first and last trade times, average duration, best pair, volume, profit factor, win/loss, win rate, expectancy and ratio, max drawdown. Refresh button |
| `/profit_long`, `/profit_short` | `[n]` | Same, filtered by direction |
| `/daily`, `/weekly`, `/monthly` | `[n]` (defaults 7, 8 and 6) | One row per period: date (trade count), absolute profit, fiat, %. Weeks start Monday. Refresh button |
| `/performance` | – | Per-pair realised profit (absolute, %, count), ranked. Refresh button |
| `/entries` (`/buys`) | `[pair]` | Performance grouped by entry tag |
| `/exits` (`/sells`) | `[pair]` | Performance grouped by exit reason |
| `/mix_tags` | `[pair]` | Performance grouped by entry-tag + exit-reason combination |
| `/stats` | – | Table of exit reason vs exits, wins and losses, plus average holding time of winners vs losers |
| `/balance` | `[full]` | Starting capital, then per-currency balance, free, used and estimated stake value. Balances under `balance_dust_level` are summed into one "other currencies" line (default from a per-stake-currency table). A dry-run warning header appears when simulating. `full` shows the whole account rather than the bot-managed part. Refresh button |

---

## C. Notifications

### C1. Message types and per-type loudness (OBSERVED-IN-CODE)

`RPCMessageType` ([FT/freqtrade/enums/rpcmessagetype.py](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/enums/rpcmessagetype.py)) defines **17 types**:

- Telegram renders 14 of them: `status`, `warning`, `exception`, `startup`, `entry`, `entry_fill`, `entry_cancel`, `exit`, `exit_fill`, `exit_cancel`, `protection_trigger`, `protection_trigger_global`, `liquidation_warning`, `strategy_msg`.
- `whitelist`, `analyzed_df` and `new_candle` are internal or websocket-only, are excluded from logging, and are ignored by Telegram.

Each type is set to one of three levels:

- **`on`**: sent with sound.
- **`silent`**: sent with `disable_notification`, so it is delivered without sound or vibration.
- **`off`**: not sent at all, and an INFO log line notes the skip.

The defaults come from the schema and the docs:

- Most types default to `on`.
- `entry_fill` defaults to `off`, and the docs say fill messages must be explicitly enabled. There is a conflict here: the schema gives `exit_fill` a default of `on`, while the docs say `*_fill` notifications are off by default (UNVERIFIED which one wins at runtime).
- Protection triggers and liquidation warnings default to `on`.

**Per-exit-reason control** ([#L639](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L639)):

- `exit` and `exit_fill` may be a string or an **object keyed by exit reason**. Examples of reasons: `roi`, `stop_loss`, `trailing_stop_loss`, `stoploss_on_exchange`, `exit_signal`, `force_exit`, `emergency_exit`, `custom_exit`, `partial_exit`, or a custom reason string.
- A `*` key sets the default for unlisted reasons. Without `*`, unlisted reasons are `on`.

**Placed vs filled.**

- `entry` and `exit` fire when the order is **placed**.
- `entry_fill` and `exit_fill` fire when the exchange reports the **fill**.
- `*_cancel` fires on cancellation, with a reason (e.g. timeout).

**Other types**

- `startup` covers several boot messages: exchange, stake, ROI, stoploss, timeframe, strategy, pairlist description and protections. A **`warning` "dry-run enabled"** message is sent first when simulating ([FT/…/rpc_manager.py#L204](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/rpc_manager.py)).
- `status` covers state changes, e.g. stopping or reloading.
- `exception` covers fatal errors with a traceback (`worker.py#L208`).
- **`strategy_msg`**: strategy code can push free text. By default each distinct text is sent **once** (deduplicated through a cache) unless the strategy asks to always send. Messages are only sent in dry-run or live mode ([FT/…/dataprovider.py#L614](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/data/dataprovider.py#L614)). They are queued during the loop and flushed to RPC once per iteration. `allow_custom_messages` gates them.
- **`show_candle`** (`ohlc`|`off`) appends the last analysed candle's OHLC to entry and exit messages.

### C2. Message formatting (OBSERVED-IN-CODE)

- **Parse mode.** Markdown is the default. HTML is used for tables in `<pre>` blocks (tables are built with `tabulate`). MarkdownV2 is used for `/logs`, with explicit escaping.
- **Entry messages:** status icon (placed vs filled), exchange name with a "(dry)" suffix, "New trade" or "Increasing position" (DCA), trade #, pair, optional candle, entry tag, amount, direction (with leverage if not 1×), open rate, current rate (placement only), and total stake in stake currency plus fiat.
- **Exit messages:**
  - A **profit-dependent icon**: one icon for ≥ 5 %, another for ≥ 0, a warning icon for a stop-loss loss, and a cross for other losses.
  - "Exiting" or "Exited", "partially" for sub-exits, and a cumulative or final profit for multi-part exits.
  - Unrealised vs realised profit %, absolute and fiat.
  - Exit reason, direction, amount, open, current, exit and close rate, and duration (h:m:s and minutes).
- **Long outputs.** These are split into several messages before reaching Telegram's 4,096-char limit (`MessageLimit.MAX_TEXT_LENGTH`). Large tables are chunked at 25 or 50 rows. One handler (`/list_custom_data`) instead **drops** an over-long message with a log warning.
- **Fiat conversion.** Amounts are converted to a display fiat currency through a CoinGecko-backed converter. This is not applicable to Tradeloom.

### C3. Inline buttons and "confirmation" (OBSERVED-IN-CODE)

- **Refresh button.** Appears on `/status table`, `/profit*`, `/daily|weekly|monthly`, `/balance`, `/performance`, `/entries`, `/exits`, `/mix_tags` and `/count`. Pressing it **edits the same message in place**, adding an "Updated:" timestamp. Callback data is a fixed string per view, such as `update_daily`.
- **Pickers for dangerous commands.** `/forceexit` and `/forcelong|short` with no args send an inline keyboard: one button per open trade or whitelist pair (1 or 3 columns), plus **Cancel**.
  - Callback data embeds the trade id or pair, e.g. `force_exit__<id>`, `force_enter__<pair>_||_<side>`.
  - On press, the handler acknowledges the callback, **edits the message to show the outcome** ("exiting trade #N…" or "canceled"), then runs the action off the event loop in a worker executor.
- **There is no explicit "are you sure?" step.**
  - The picker selection *is* the confirmation.
  - `/forceexit <id>` and `/forceexit all` with arguments run immediately.
  - `/delete` and `/coo` also run immediately.
  - Tradeloom should be stricter (§F, FR-TG-020).

### C4. Custom reply keyboard

- The default is a three-row keyboard of common read-only and start/stop commands, sent as a `ReplyKeyboardMarkup` (resized) on every plain message.
- Users can define their own rows. At startup, each entry is validated against an allowlist of regexes. Only argument-less commands are allowed, with a few fixed-argument forms such as `/daily 7`, `/status table` and `/marketdir long`.
- Any invalid entry **aborts startup** with a config error naming the bad keys.
- The allowlist includes `/forceexit` and `/forcelong`, but only argument-less, so they open the picker rather than executing.

### C5. Balance dust level

`/balance` hides currencies whose estimated stake value is ≤ `balance_dust_level`. It still shows open positions, and it summarises hidden ones as "N other currencies (< level): total". The default comes from a per-stake-currency table (`DUST_PER_COIN`), falling back to 1.0. This is crypto-specific (§D2).

### C6. Rate limits and errors (freqtrade side)

- **Outbound sends** ([#L2195](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/telegram.py#L2195)) are sent to the configured chat and topic. On a `NetworkError`, the send is **retried exactly once**. Any other `TelegramError` is logged and the message is **dropped**.
- **No rate limiter or 429 back-off** is implemented in freqtrade's Telegram module. It relies on low message volume (UNVERIFIED whether python-telegram-bot's optional rate limiter is enabled; the builder call at #L254 does not configure one).
- **Telegram's published limits** ([Bots FAQ](https://core.telegram.org/bots/faq)):
  - about **1 message/second per chat** (short bursts tolerated);
  - **20 messages/minute in a group**;
  - about **30 messages/second overall** for free bots.

  Exceeding them returns **HTTP 429** with `retry_after` seconds to wait ([Bot API, ResponseParameters](https://core.telegram.org/bots/api)).
- **Telegram's published size limits** ([Bot API](https://core.telegram.org/bots/api)):
  - message text is 1–4096 characters after entity parsing;
  - `callback_data` is **1–64 bytes**;
  - the client shows a progress spinner until the bot calls **`answerCallbackQuery`**, so every button press must be answered;
  - `getUpdates` and webhooks are mutually exclusive, and the server keeps undelivered updates for **at most 24 hours**.

---

## D. Architecture (freqtrade)

### D1. RPC layer decoupling

- **`RPC` class** ([FT/…/rpc.py#L115](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/rpc.py#L115)). This is a transport-neutral **query/command facade** over the engine: `_rpc_trade_status`, `_rpc_status_table`, `_rpc_timeunit_profit`, `_rpc_trade_statistics`, `_rpc_balance`, `_rpc_start/stop/pause/reload_config`, `_rpc_force_exit/force_entry`, `_rpc_cancel_open_order`, `_rpc_delete`, `_rpc_locks`, `_rpc_whitelist/blacklist`, `_rpc_get_logs`, `health`, and others.
  - Telegram, the REST API (FreqUI) and Discord or webhook handlers all call the **same** methods. Business rules therefore live in one place, e.g. "refuse if STOPPED", the exit lock, and order cancellation.
  - Errors surface as `RPCException`, which each transport turns into a user message.
- **`RPCHandler` base**: `name`, `send_msg(msg)`, `cleanup()`. Implementations are Telegram, Discord, Webhook and ApiServer.
- **`RPCManager`** ([FT/…/rpc_manager.py](https://github.com/freqtrade/freqtrade/blob/d636f8e472f5990b1df4a22046ffc3553c9b4eb9/freqtrade/rpc/rpc_manager.py)) builds the enabled handlers from config. `send_msg` fans each **typed message dict** out to every handler.
  - **Message schemas** are typed dicts in `rpc_types.py`: status, strategy, protection, liquidation, entry, cancel, exit, exit-cancel, and so on. Each carries trade id, pair, rates, amounts, profit, exit reason and dates.
  - The engine emits *facts*. Each handler decides formatting and loudness.

### D2. Threading and queues

- **Outbound delivery.** Each handler with `_use_queue = True` (the default; the API server opts out) gets its own **unbounded `Queue` and a daemon worker thread**.
  - The trading loop only enqueues a shallow copy, so a slow endpoint can never stall trading.
  - A warning is logged every time a queue backlog reaches a multiple of 100.
  - Delivery errors are caught per handler and logged.
- **Shutdown.** A stop sentinel is enqueued and there is a **10 s total grace period** to drain. After that, pending messages are discarded with a warning and the in-flight send gets another 10 s.
- **Telegram internals.**
  - The module runs python-telegram-bot's asyncio `Application` on **its own thread with its own event loop**.
  - Outbound `send_msg` (from the RPC worker thread) hands the coroutine to that loop with `run_coroutine_threadsafe`.
  - Inbound command handlers run on the Telegram loop. Engine-mutating calls that block (force exit, force entry) are pushed to an executor, so the polling loop stays responsive.
  - The DB session is scoped per call.
- **Concurrency safety.** Engine-mutating RPC methods take the bot's **exit lock** (`_exit_lock`), so a Telegram force-exit cannot race the trading loop's own exit logic.

### D3. Error handling summary

| Situation | Behaviour |
|---|---|
| Telegram init fails | 3 tries, then continue without Telegram |
| Network error on send | 1 retry, then drop |
| Other Telegram API error | Log a warning and drop |
| "Message not modified" on Refresh | Ignored |
| Business rule violation | `RPCException` text replied to the user |
| Unexpected exception in a handler | Logged and swallowed |
| Unknown message type | Logged at DEBUG and skipped |
| Handler not implementing a type | Logged as an error by the manager |

---

## E. Go library choice (verified 2026-10-12)

| Library | Latest | Licence | Maintenance / notes |
|---|---|---|---|
| **`github.com/go-telegram/bot`** | **v1.27.0** (2026-09-11, proxy.golang.org) | **MIT** | Active; last push 2026-10-08. README says "zero-dependencies", and the module's `go.mod` has no `require` lines. Supports Bot API **10.3**. Has long polling (`Start(ctx)`), webhooks, middleware, callback-query data handlers and `AnswerCallbackQuery`. 384 importers on pkg.go.dev |
| `github.com/mymmrac/telego` | v1.12.1 (2026-08-29) | MIT | Active, but pulls in `bytedance/sonic`, `valyala/fasthttp` and others; 224 importers |
| `github.com/go-telegram-bot-api/telegram-bot-api/v5` | v5.5.1 (**2021-12-13**) | MIT | Most-imported (2,572) but **no release since 2021**. Its Bot API coverage is stale, so it is not recommended |
| `gopkg.in/telebot.v4` | v4.0.0-beta.10 (2026-06-16) | GitHub reports NOASSERTION | Still beta, and the licence detection is unclear |

**Recommendation: `github.com/go-telegram/bot` v1.27.0 (MIT).**

- It has no transitive dependencies, which keeps the Wails binary and the licence audit simple.
- It is current with the Bot API and context-based, which fits Go cancellation on app shutdown.
- It has native callback-query routing.

Sources: [pkg.go.dev/github.com/go-telegram/bot](https://pkg.go.dev/github.com/go-telegram/bot), [proxy.golang.org …/@latest](https://proxy.golang.org/github.com/go-telegram/bot/@latest), [GitHub releases](https://github.com/go-telegram/bot/releases), [pkg.go.dev telego](https://pkg.go.dev/github.com/mymmrac/telego), and [pkg.go.dev telegram-bot-api/v5](https://pkg.go.dev/github.com/go-telegram-bot-api/telegram-bot-api/v5). Long polling, rather than webhooks, is the right transport. A local desktop app has no public HTTPS endpoint, and polling needs only outbound HTTPS to `api.telegram.org`.

---

## F. Mapping freqtrade → Tradeloom

### F1. Principles that shape the mapping

1. **Telegram is a remote control for the running desktop app, not a separate server.**
   - When the app is closed, suspended or offline, the bot is offline.
   - GTT-protected exits keep working on Zerodha's side regardless (see A07 A8b), so the startup and shutdown messages must say so.
2. **Every Kite action happens from the desktop process.** This means the user's static IP, the 10 OPS throttle and `market_protection`. A Telegram confirmation is just another *user confirmation* input into the same order path as the in-app button. No new order path is created.
3. **Paper vs live is always visible.**
   - Every trade message carries a `PAPER` or `LIVE` tag. This is the freqtrade "(dry)" suffix, generalised per deployment.
   - Paper deployments never touch Kite.
4. **Asymmetric safety.**
   - Actions that **reduce risk** can be done remotely with one confirmation: pause entries, disarm auto-exec, engage the kill switch, exit a position, cancel a pending entry.
   - Actions that **add risk** are blocked by default and need an explicit settings opt-in plus confirmation: confirm a signal into an order, arm auto-exec, resume after the kill switch.
   - Creating or editing strategies, logging in, changing settings and disengaging the kill switch are **desktop-only**.
5. **Stricter than freqtrade on confirmations.**
   - Commands that place or cancel orders always use a two-step **Confirm / Cancel** inline prompt.
   - The prompt is bound to the requesting user, single-use, expires (default 60 s), and is re-validated against live state when pressed.
   - Every callback is authorized.
6. **Kite login cannot be done from Telegram.**
   - The redirect goes to the desktop's loopback listener, and the research says the daily login must be manual (A07 A2).
   - Telegram can only *remind* the user and report session status. The token expires 06:00 IST.

### F2. Command mapping

Legend:

- **R** = read-only.
- **S** = state change, no Kite order.
- **O** = places, modifies or cancels a Kite order (live) or a simulated order (paper).
- "Confirm" = two-step inline Confirm/Cancel (FR-TG-020).

| freqtrade | Tradeloom command | Kind | What it does in Tradeloom | Data source | Safety rule |
|---|---|---|---|---|---|
| `/help` | `/help` | R | Grouped list of commands available **under current settings**. Hidden: order-capable commands that are disabled | Static plus settings | – |
| `/version` | `/version` | R | App version, build, Kite API version | App | – |
| `/health` | `/health` | R | Engine heartbeat, last tick time per feed, ticker WebSocket state, last Kite REST success, queue depth, app uptime | Engine | – |
| (none) | `/session` | R | Kite session: logged in or not, user id (masked), token expiry (06:00 IST), static-IP check result | Kite `GET /user/profile` and app | – |
| `/show_config` | `/settings` | R | Redacted summary: mode per deployment (paper or live), auto-exec armed or not, kill-switch state, Telegram permissions, risk limits | App settings | Never shows secrets |
| `/logs [n]` | `/logs [n]` | R | Last *n* app log lines at WARN and above by default. Chunked, max 50 | App log ring buffer | Redacts tokens |
| `/status` | `/status` | R | All **active deployments**, one block each: strategy, instrument, state (ARMED / ENTRY_PENDING / IN_POSITION / EXIT_PENDING / DAY_DONE / EXPIRED / ATTENTION), PAPER/LIVE, position qty, avg price, LTP, MTM P&L, SL/target, GTT status | Engine, Kite ticks and GTT | – |
| `/status <id…>` | `/status <id…>` | R | The same for specific deployment ids | Engine | – |
| `/status table` | `/status table` | R | Compact table: id, instrument, state, side, qty, P&L. Refresh button | Engine | – |
| `/order <id>` | `/orders [id]` | R | Today's orders, either all or for one deployment: Kite order id, type, status, fill price, rejection reason | Kite `GET /orders`, local journal | – |
| (none) | `/gtt` | R | Active GTTs placed by Tradeloom: instrument, trigger, status | Kite `GET /gtt/triggers` | – |
| `/balance` | `/funds` | R | Equity margin: available cash, used margin, net. Paper: the virtual capital ledger | Kite `GET /user/margins` | No dust concept (INR only) |
| (none) | `/positions` | R | Kite net and day positions, flagged *managed by Tradeloom* vs *external* | Kite `GET /portfolio/positions` | – |
| (none) | `/holdings` | R | Demat holdings summary (top N by value, plus total) | Kite `GET /portfolio/holdings` | – |
| `/count` | `/count` | R | Deployments per state, live vs paper, open positions vs the configured max | Engine | – |
| `/trades [n]` | `/trades [n]` | R | Last *n* closed round-trips: instrument, strategy, P&L ₹ and %, exit reason, PAPER/LIVE | Local trade journal | – |
| `/profit [n]` | `/profit [n] [live\|paper] [long\|short]` | R | Realised P&L, trade count, win rate, profit factor, expectancy, average holding time, max drawdown, best and worst instrument. Long/short matters because MIS shorts and futures shorts exist | Journal | – |
| `/daily /weekly /monthly` | same | R | Per-period P&L table, IST calendar. Weeks start Monday | Journal | – |
| `/performance` | `/performance` | R | P&L by instrument | Journal | – |
| `/entries` `/exits` `/mix_tags` | `/bystrategy` `/exits` `/mix` | R | P&L grouped by strategy (the closest analogue of entry tag), by exit reason (target, SL, trailing SL, square-off time, exit rule, manual, kill switch, GTT trigger), and by both | Journal | – |
| `/stats` | `/stats` | R | Exit-reason win/loss table, plus average holding time for winners vs losers | Journal | – |
| `/locks` | `/attention` | R | Deployments in ATTENTION, DAY_DONE (with reason) or EXPIRED | Engine | – |
| `/unlock` | (desktop-only) | – | Clearing ATTENTION needs the user to look at the reconciliation in the app | – | Not remote |
| (none) | `/alerts` | R | Active scanner alerts and price alerts, plus the last trigger time | Scanner engine | – |
| (none) | `/signals` | R | Pending notification-based signals awaiting confirmation, with expiry countdown | Engine | – |
| (none, Tradeloom-specific) | inline **Place / Skip** on a signal message | O | Confirms a pending NBT signal into an order (live) or a simulated order (paper) | Engine → Kite `POST /orders` | **Off by default** (`allow_telegram_order_confirm`). Re-validates the signal, price drift, margin, session, kill switch and static IP at press time. Single-use. Expires with the signal |
| `/pause` / `/stopentry` | `/pause [id]` | S | No args: global **entry pause**. No deployment may enter, while open positions keep being managed (SL, target, GTT, square-off). With an id: pause that deployment only | Engine | Allowed (reduces risk), with one confirm for global |
| `/start` | `/resume [id]` | S | Lift the pause (global or per deployment) | Engine | Allowed only if `allow_telegram_resume` is set (default **off**); confirm required. Blocked while the kill switch is engaged |
| `/stop` | (no direct equivalent) | – | Stopping the engine while positions are open would leave them unmanaged. Use `/pause` or `/kill` instead | – | – |
| (none) | `/kill` | O | **Kill switch**: disarm auto-exec; pause all deployments; cancel all open *entry* orders; and, depending on settings, square off all Tradeloom-managed positions (MARKET with protection) or leave them under their GTT/SL | Engine and Kite | Always available to authorized users. Confirm required (one tap). **Disengage is desktop-only** by default |
| `/forceexit <id>\|all` | `/exit <id>\|all` | O | Exit a managed position now: cancels the existing GTT/SL legs first, then places the exit order (paper: simulated) | Engine → Kite | Confirm required, showing instrument, qty, LTP and est. P&L. `all` lists every position being closed. Allowed by default (reduces risk); can be disabled |
| `/cancel_open_order` | `/cancel <id>` | O | Cancel a pending **entry** order of a deployment. Cancelling a protective exit or GTT is **not** offered remotely | Kite `DELETE /orders` | Confirm required |
| `/reload_trade` | `/reconcile <id>` | S | Re-read the deployment's orders and positions from Kite and repair the local state. If inconsistent, it moves to ATTENTION rather than guessing | Kite orders, positions, GTT | Allowed; no confirm |
| (none) | `/disarm` | S | Disarm auto-execution globally (signals go back to notify-and-confirm) | Engine | Always allowed; confirm |
| (none) | `/arm` | S | Arm auto-execution | Engine | **Blocked unless `allow_remote_arming` = true** (default false). Even then: static-IP check must pass, Kite session valid, kill switch off. Confirm by typing a code shown in the message (two-step) |
| `/forcelong` / `/forceshort` | (not in v1) | O | A manual discretionary entry from a phone, outside any strategy | – | **Won't (v1)**. The risk and SEBI-algo framing are unclear, and it is easy to fat-finger. Reconsider later |
| `/delete` | (desktop-only "detach") | – | Removing a trade from tracking without touching the broker is a reconciliation action and needs the full UI | – | Not remote |
| `/reload_config` | (n/a) | – | Tradeloom settings are edited in the UI and applied live. There is no config file to reload | – | – |
| `/whitelist` | `/deployments` (alias of `/status table` filtered to ARMED) | R | What the engine is currently watching | Engine | – |
| `/blacklist` `/bl_delete` | (n/a) | – | No pairlist concept. Instrument eligibility comes from deployments and scanner universes | – | – |
| `/marketdir` | (n/a) | – | A strategy-readable manual "market bias" variable has no Tradeloom equivalent. It could later map to a user variable, but it hurts backtest reproducibility (freqtrade warns about this too) | – | – |
| `/list_custom_data` | `/status <id> detail` | R | Shows the deployment's rule-evaluation state (last evaluated values per condition) | Engine | – |
| `/tg_info` | setup **pairing code** | – | Replaced by a one-time pairing flow started from the desktop (FR-TG-003), so no unauthenticated command needs to stay enabled permanently | – | Only active during a 10-min pairing window |
| (none) | `/digest` | R | Today's summary on demand (same content as the end-of-day digest) | Journal, Kite | – |

### F3. Notification mapping

| freqtrade type | Tradeloom type(s) | Notes |
|---|---|---|
| `startup` | `app_online` | App started or Telegram connected. Includes mode summary, armed or not, kill switch, Kite session state, how many commands were dropped while offline |
| (status on shutdown) | `app_offline` | Graceful quit or suspend: "Bot offline; GTT protections remain at Zerodha; no signals or alerts until reopened". Not possible on crash or power loss (FR-TG-010) |
| `status` | `engine_status` | Pause and resume, arm and disarm, settings changes |
| `warning` | `warning` | Ticker disconnect or reconnect, Kite 429 or throttling, static-IP mismatch, margin insufficient, GTT placement failure, order rejection |
| `exception` | `error` | Unexpected engine error (redacted) |
| (none) | `session` | Login reminder before market, login succeeded, token expired at 06:00, Kite returned `TokenException` |
| (none) | `signal` | NBT signal awaiting confirmation (with Place/Skip buttons if enabled). The core Tradeloom use-case |
| `entry` | `entry` | Entry order placed: deployment, instrument, side, qty, price or type, product (MIS/CNC/NRML), PAPER/LIVE |
| `entry_fill` | `entry_fill` | Filled: avg price, plus the GTT/SL protection placed |
| `entry_cancel` | `entry_cancel` | Cancelled or rejected or expired, with reason (Kite status message) |
| `exit` (per reason) | `exit` (per reason) | Reasons: `target`, `stop_loss`, `trailing_stop`, `exit_rule`, `square_off_time`, `manual` (desktop or Telegram), `kill_switch`, `gtt_triggered`, `expiry` (futures expiry roll or close) |
| `exit_fill` | `exit_fill` | Realised P&L ₹ and %, duration |
| `exit_cancel` | `exit_cancel` | Exit order not completed. **Always `on`**, and cannot be silenced, because it can leave risk open |
| `protection_trigger` | `risk_guard` | Per-deployment guard tripped (e.g. max trades per day reached → DAY_DONE) |
| `protection_trigger_global` | `risk_guard_global` | Daily loss limit or max drawdown hit → global pause or kill switch |
| `liquidation_warning` | (n/a; nearest is `margin_warning`) | No leverage liquidation in cash equity. For futures, a **margin shortfall** warning from Kite margins is the useful analogue |
| (none) | `state_attention` | A deployment entered ATTENTION (needs human). **Always `on`** |
| (none) | `state_change` | DAY_DONE and EXPIRED transitions (default `silent`) |
| (none) | `kill_switch` | Engaged or disengaged, with actions taken. **Always `on`** |
| (none) | `scanner_alert` | Scanner hit (instrument, scanner, timeframe, trigger price), per-scanner toggle |
| (none) | `price_alert` | User price alert hit |
| (none) | `backtest_done` | Long backtest finished: summary metrics. Default `silent` |
| `strategy_msg` | `strategy_msg` | Optional user-authored text from strategy rules, deduplicated per candle |
| (none) | `daily_digest` | End-of-day summary at a configurable time (default 15:45 IST) |

### F4. Not applicable, and why

- **Stake amount and stake currency, fiat conversion, `balance_dust_level`.** Tradeloom is INR-only, with no multi-coin wallet. Position sizing is per deployment in qty or ₹. `/funds` shows Kite margins directly.
- **Leverage, liquidation, margin mode (isolated/cross), funding fees.** These are crypto-perpetual concepts. Indian cash and futures have SPAN plus exposure margin, and Kite does no bot-side liquidation stop. The only analogue is a margin-shortfall warning.
- **Pairlists, whitelist and blacklist semantics.** freqtrade picks pairs dynamically from exchange volume lists and blacklists by pattern. Tradeloom trades exactly what the user deployed per instrument, so there is nothing to blacklist remotely.
- **`/forceshort` on spot vs futures distinction.** Shorting in Tradeloom is a property of the strategy and product (MIS short or futures short), not a separate command.
- **`/reload_config`.** There is no config file; settings apply live from the UI.
- **`/delete`.** Silently forgetting a broker-side position is unsafe. It is replaced by a desktop-only reconciliation action.
- **`/marketdir`.** A non-persisted manual bias variable makes backtests non-reproducible.
- **`show_candle`.** Can be kept as an option ("include last candle OHLC"), but it is low value.
- **24/7 operation assumptions.** Crypto never closes. Tradeloom must be market-hours aware: NSE 09:15–15:30 IST, with pre-open and post-close. It also needs session-expiry handling (06:00 IST), which crypto bots do not have.

---

## G. Draft functional requirements (module TG)

Priority hints use MoSCoW for v2 scoping.

### FR-TG-001 Telegram integration is optional and off by default
**Requirement.** The app shall include a Telegram integration that is disabled on first install and can be enabled from Settings → Telegram. When disabled, the app shall make no network calls to `api.telegram.org`.

**Acceptance criteria.**
- Given a fresh install, when the app starts, then Telegram is disabled and no request is made to `api.telegram.org`.
- Given Telegram is enabled with valid credentials, when the user disables it, then polling stops within 25 s and no further messages are sent.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-002 Secure storage of the bot token
**Requirement.** The bot token shall be stored in the OS secret store (Linux Secret Service / keyring). It shall be stored in no other place: not in plain config files, logs, crash reports or Telegram messages. The UI shall show it masked, with only the last 4 characters visible.

**Acceptance criteria.**
- Given a saved token, when the config directory and logs are searched for the token string, then there are no matches.
- Given the secret store is unavailable, when the user tries to save a token, then the app refuses and explains why. It does not fall back to plaintext.

**Data/Kite dependency.** OS keyring (the Go keyring library is still to be chosen; UNVERIFIED).
**Priority hint.** Must.

### FR-TG-003 Guided setup and pairing
**Requirement.** Settings shall offer a guided setup:
1. Instructions for creating a bot with @BotFather.
2. Paste and validate the token (call `getMe` and show the bot username).
3. **Pairing:** the app shows a one-time 6-digit code valid for 10 minutes. The user sends `/pair <code>` to the bot from the private chat or group/topic to be used.
4. The app records that chat's `chat_id`, `topic_id` and the sender's `user_id` as the first authorized user.

No unauthenticated command shall be answered outside an active pairing window.

**Acceptance criteria.**
- Given pairing is active, when `/pair <correct code>` arrives, then `chat_id`, `topic_id` and `user_id` are saved and a confirmation is sent to that chat.
- Given pairing is not active, when `/pair` or any other command arrives from an unknown chat, then nothing is replied.
- Given 5 wrong codes, when another attempt arrives, then the pairing window is closed and the desktop shows a warning.

**Data/Kite dependency.** Telegram `getMe` and `getUpdates`.
**Priority hint.** Must.

### FR-TG-004 Authorization of every update
**Requirement.** Every inbound command **and every callback query** shall be processed only if all of these hold:
- the chat id equals the configured chat;
- the topic id equals the configured topic, if one is set;
- the sender's user id is in the authorized-user list.

The list shall always be non-empty once paired. Confirmation callbacks shall additionally be accepted only from the user who requested the action.

**Acceptance criteria.**
- Given a group where user B is a member but not authorized, when B presses a Confirm button that user A requested, then nothing executes and the callback is answered with "Not authorized".
- Given a message from another chat, when it is a valid command, then it is ignored.

**Data/Kite dependency.** Telegram updates.
**Priority hint.** Must.

### FR-TG-005 Handling unauthorized messages
**Requirement.** Unauthorized commands shall get no chat reply. Callback queries are the exception: they are answered silently so the client spinner stops. Each rejection shall be written to the security log with chat id, user id and command name, but not the message body. More than 10 rejections in 10 minutes shall raise one in-app warning, and a Telegram `warning` to the authorized chat.

**Acceptance criteria.**
- Given 11 unauthorized commands within 10 minutes, then exactly one in-app warning and one Telegram warning are produced, and none of the commands is executed.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-006 Group and topic use
**Requirement.** The bot may be paired to a private chat (recommended) or to a group, optionally a forum topic. When the target is a group, Settings shall show a warning that all authorized users can act on the account and that every group member can read notifications. Settings shall allow adding and removing authorized user ids.

**Acceptance criteria.**
- Given pairing from a group, when it completes, then the warning is shown and must be acknowledged before saving.
- Given a topic is configured, when a notification is sent, then it is posted in that topic.

**Data/Kite dependency.** None.
**Priority hint.** Should.

### FR-TG-007 Connection status and test message
**Requirement.** Settings shall show the Telegram connection state:
- disabled;
- connecting;
- online (with last successful poll time);
- error (with last error).

A **Send test message** button shall be provided.

**Acceptance criteria.**
- Given an invalid token, when the integration starts, then the status shows "error: unauthorized token" and the app keeps running normally.
- Given online, when Send test is clicked, then a test message arrives in the paired chat.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-008 Lifecycle bound to the desktop app (offline behaviour)
**Requirement.** The bot shall run only while the Tradeloom desktop process is running. It shall use long polling (`getUpdates`); webhooks are not used. When the app is closed, suspended, or has no network, the bot is offline: no commands are processed, no notifications are sent, and Telegram keeps undelivered commands for at most 24 h. Settings and docs shall state this clearly, together with the fact that GTT protections at Zerodha remain active while offline. Telegram failures shall never block or stop trading.

**Acceptance criteria.**
- Given the app is quit, when the user sends `/status`, then there is no response. The docs say this is expected.
- Given `api.telegram.org` is unreachable, when a signal fires, then the in-app/OS notification still appears and the engine's timing is unaffected.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-009 Discard stale commands on start
**Requirement.** On every (re)connect, the app shall fetch and acknowledge pending updates **without executing them**. It shall then send one message listing the dropped commands (name and time received). Pending callback presses on old confirmation prompts shall also be discarded.

**Acceptance criteria.**
- Given `/exit all` was sent while the app was closed, when the app starts, then no exit is placed and the startup message says "1 command received while offline was ignored: /exit all (08:02)".

**Data/Kite dependency.** Telegram `getUpdates` offset.
**Priority hint.** Must.

### FR-TG-010 Online/offline announcements
**Requirement.** The app shall send:
- **`app_online`** when Telegram connects. It includes app version; per-mode deployment counts (live/paper); auto-exec armed state; kill-switch state; Kite session state; and dropped-command info (FR-TG-009).
- **`app_offline`** on graceful quit or system suspend, within a 5 s grace period. It says that signals and alerts stop and GTTs remain.

Crash or power loss cannot be announced; the docs shall say so.

**Acceptance criteria.**
- Given a normal quit, then `app_offline` arrives before the process exits, unless the network is down.
- Given a crash, then on next start `app_online` includes "previous session ended unexpectedly at <time>".

**Data/Kite dependency.** Kite session status.
**Priority hint.** Should.

### FR-TG-011 Command menu and /help
**Requirement.** The app shall register its command list with Telegram (`setMyCommands`), so the client shows a menu. `/help` shall list commands grouped as Status, Account, Performance and Control. It shall omit commands disabled by current permissions and mark order-capable commands with "requires confirmation".

**Acceptance criteria.**
- Given `allow_telegram_order_confirm` is off, when `/help` is sent, then signal confirmation is described as disabled.
- Given a permission changes, then the command menu is updated within 60 s.

**Data/Kite dependency.** None.
**Priority hint.** Should.

### FR-TG-012 Deployment status commands
**Requirement.** The bot shall provide:
- `/status`: one block per active deployment with id, strategy, instrument, state, PAPER/LIVE tag, position qty and avg price, LTP, MTM P&L ₹ and %, SL and target, GTT status, time in state.
- `/status <id…>`: the listed deployments only.
- `/status table`: a monospaced table of id, instrument, state, side, qty and P&L.
- `/status <id> detail`: the last evaluated value of each rule condition.

Output longer than 4096 characters shall be split across messages.

**Acceptance criteria.**
- Given 120 deployments, when `/status table` is sent, then the table arrives in at most 3 messages, none exceeding 4096 characters.
- Given an unknown id, then the reply is "Deployment <id> not found".

**Data/Kite dependency.** Engine state, Kite ticks for LTP.
**Priority hint.** Must.

### FR-TG-013 Account commands
**Requirement.** The bot shall provide:
- `/funds`: equity available, used, net from Kite margins, or the paper ledger.
- `/positions`: Kite net positions, each flagged as Tradeloom-managed or external.
- `/holdings`: top 10 holdings by value, plus total value and P&L.

Kite errors shall be reported as a short message, e.g. "Kite session expired — log in on the desktop".

**Acceptance criteria.**
- Given the Kite token has expired, when `/positions` is sent, then the reply explains that login is required and no stack trace is shown.

**Data/Kite dependency.** `GET /user/margins`, `GET /portfolio/positions`, `GET /portfolio/holdings`.
**Priority hint.** Must.

### FR-TG-014 Orders and GTT commands
**Requirement.** The bot shall provide:
- `/orders [id]`: today's orders, all or for one deployment, with status and rejection reason.
- `/gtt`: active Tradeloom-created GTTs with trigger values.

**Acceptance criteria.**
- Given a rejected order today, then `/orders` shows it with Kite's status message.

**Data/Kite dependency.** `GET /orders`, `GET /gtt/triggers`.
**Priority hint.** Should.

### FR-TG-015 Performance commands
**Requirement.** The bot shall provide these, all from the local trade journal, with `live|paper` and `long|short` filters where meaningful:
- `/trades [n]` (default 10, max 50)
- `/profit [n] [live|paper] [long|short]`: realised P&L, count, win rate, profit factor, expectancy, avg holding time, max drawdown, best and worst instrument
- `/daily [n]` (default 7)
- `/weekly [n]` (8, weeks start Monday)
- `/monthly [n]` (6)
- `/performance` (by instrument)
- `/bystrategy`
- `/exits` (by exit reason)
- `/mix` (strategy × exit reason)
- `/stats` (exit-reason wins/losses and average holding time of winners vs losers)

Paper and live shall never be mixed unless the user explicitly asks for `all`.

**Acceptance criteria.**
- Given both paper and live trades, when `/profit` is sent, then it shows live only by default and says so in the header.
- Given `/daily abc`, then the default (7) is used.

**Data/Kite dependency.** Local journal (no Kite calls).
**Priority hint.** Should.

### FR-TG-016 Engine and session information commands
**Requirement.** The bot shall provide:
- `/health`: last tick time, ticker WebSocket state, last Kite REST success, notification queue depth, uptime.
- `/session`: Kite login state, masked user id, token expiry time (06:00 IST), static-IP check result.
- `/version`.
- `/settings`: redacted mode, arming, kill switch, Telegram permissions, risk limits.
- `/logs [n]`: WARN and above, max 50, secrets redacted.

**Acceptance criteria.**
- Given `/settings`, then no API key, API secret, access token or bot token appears in the output.

**Data/Kite dependency.** `GET /user/profile`, app state.
**Priority hint.** Should.

### FR-TG-017 Attention, signals and alerts listing
**Requirement.** The bot shall provide:
- `/attention`: deployments in ATTENTION, DAY_DONE or EXPIRED, with reasons.
- `/signals`: pending signals with time left.
- `/alerts`: active scanner and price alerts with last trigger.

**Acceptance criteria.**
- Given a deployment in ATTENTION, then `/attention` shows it with the reason and the hint "resolve on desktop".

**Data/Kite dependency.** Engine.
**Priority hint.** Should.

### FR-TG-018 Refresh buttons with throttling
**Requirement.** Table-style replies shall carry an inline **Refresh** button that edits the message in place and appends "Updated HH:MM:SS IST". This applies to `/status table`, `/profit`, `/daily`, `/weekly`, `/monthly`, `/performance`, `/funds`, `/positions` and `/count`.
- Presses shall be throttled to one refresh per message per 3 s. A press inside that window is answered with "Too fast".
- "Message not modified" errors shall be ignored.
- The buttons can be turned off in Settings.

**Acceptance criteria.**
- Given two Refresh presses 1 s apart, then only one Kite or engine query is made and the second press shows "Too fast".

**Data/Kite dependency.** As per the underlying command.
**Priority hint.** Could.

### FR-TG-019 Custom reply keyboard
**Requirement.** Settings shall allow a custom reply keyboard (rows of command buttons). Only argument-less **read-only** commands, plus `/pause` and `/kill`, may be placed on it. Any other entry is rejected **at save time** with the offending entries named. Unlike freqtrade, the app does not fail at startup.

**Acceptance criteria.**
- Given the user adds `/exit all` to the keyboard, when saving, then validation fails and names that entry.

**Data/Kite dependency.** None.
**Priority hint.** Could.

### FR-TG-020 Confirmation framework for risky actions
**Requirement.** Every command of kind **O**, and the global S-commands marked "confirm" in §F2, shall send a confirmation prompt.
- **Prompt content:** what will happen, listing instruments, qty, side, LTP, estimated P&L or value, and PAPER/LIVE.
- **Buttons:** **Confirm** and **Cancel**.
- **Callback data:** an opaque single-use nonce of 64 bytes or less, mapped server-side to the action, the requesting user id and an expiry (default 60 s, configurable 15–300 s).
- **On Confirm:**
  1. re-check authorization and the requesting user;
  2. re-validate live state: session, kill switch, deployment state, position still open, price drift within tolerance;
  3. execute **once**;
  4. answer the callback;
  5. edit the prompt to show the outcome, with the Kite order id or the error.
- **Expired, used or cancelled prompts** are edited to say so, and their buttons are removed.

**Acceptance criteria.**
- Given a prompt pressed twice quickly, then exactly one order is placed.
- Given a prompt pressed after 61 s, then nothing executes and the message says "expired".
- Given the position was already closed by GTT before Confirm, then nothing is sent to Kite and the reply says "already flat".

**Data/Kite dependency.** Engine; Kite order APIs.
**Priority hint.** Must.

### FR-TG-021 Confirm notification-based signals from Telegram
**Requirement.** When `allow_telegram_order_confirm` is on (default **off**), each `signal` notification shall carry **Place** and **Skip** buttons. Place runs the same order path as the in-app confirmation: static IP, ≤10 OPS throttle, `market_protection` for MARKET, product per strategy, then GTT protection after fill. It also checks:
- the signal is still pending and not expired;
- LTP drift from the signal price is within the strategy's tolerance (default 0.5 %);
- sufficient margin;
- valid Kite session;
- kill switch off;
- auto-exec not already handling it.

Confirming from either the desktop or Telegram resolves the signal everywhere. In paper mode, Place creates a simulated order.

**Acceptance criteria.**
- Given the setting is off, then signal messages have no buttons and say "confirm in app".
- Given a signal confirmed on the desktop, when Place is pressed in Telegram, then the reply is "already confirmed" and no second order is placed.
- Given LTP moved 1 % against a 0.5 % tolerance, then the order is not placed and the user is told the reason.

**Data/Kite dependency.** `POST /orders/regular`, `GET /user/margins`, GTT APIs, static-IP check.
**Priority hint.** Should.

### FR-TG-022 Exit positions remotely
**Requirement.** `/exit <id>` and `/exit all` shall, after confirmation (FR-TG-020):
1. cancel the deployment's protective GTT and pending SL orders;
2. place the exit order (MARKET with protection, or LIMIT at LTP ± configurable buffer);
3. report the fill.

`/exit` with no args shows a picker of open managed positions, and each pick then goes through confirmation. External (non-Tradeloom) positions are never touched. Exits can be disabled in Settings (`allow_telegram_exit`, default on).

**Acceptance criteria.**
- Given 3 managed and 1 external position, when `/exit all` is confirmed, then exactly 3 exit orders are placed, and the reply lists them and notes that the external one was skipped.
- Given GTT cancellation fails, then the exit is not placed, the deployment goes to ATTENTION, and an always-on alert is sent.

**Data/Kite dependency.** `DELETE /gtt/triggers/{id}`, `POST /orders`, `GET /orders`.
**Priority hint.** Must.

### FR-TG-023 Cancel a pending entry order
**Requirement.** `/cancel <id>` shall, after confirmation, cancel the deployment's open **entry** order and return the deployment to ARMED or DAY_DONE according to its rules. Cancelling protective exits or GTTs from Telegram shall not be possible.

**Acceptance criteria.**
- Given a deployment in IN_POSITION with a GTT, when `/cancel <id>` is sent, then the reply explains that only pending entry orders can be cancelled remotely.

**Data/Kite dependency.** `DELETE /orders/regular/{id}`.
**Priority hint.** Should.

### FR-TG-024 Pause and resume entries
**Requirement.**
- `/pause` (global, confirm) and `/pause <id>` (one deployment) shall stop new entries while open positions stay managed.
- `/resume [id]` shall be available only if `allow_telegram_resume` is on (default off). It needs confirmation and is refused while the kill switch is engaged.
- Pause state shall persist across app restarts. This differs from freqtrade.

**Acceptance criteria.**
- Given a global pause, when an entry condition becomes true, then no signal or order is produced, while SL/target exits still execute.
- Given the app restarts, then the pause is still in effect.

**Data/Kite dependency.** Engine.
**Priority hint.** Must.

### FR-TG-025 Kill switch from Telegram
**Requirement.** `/kill` shall be available to every authorized user and needs one confirmation. It engages the same kill switch as the desktop:
- disarm auto-exec;
- pause all deployments;
- cancel all open entry orders;
- then, per the kill-switch setting, either square off all Tradeloom-managed positions or leave them under existing GTT/SL.

It then reports a per-action summary. **Disengaging the kill switch shall be desktop-only**, unless `allow_telegram_kill_reset` is enabled (default off).

**Acceptance criteria.**
- Given `/kill` is confirmed, then a `kill_switch` notification lists every cancelled order and exit placed, and `/status` shows all deployments paused.
- Given the kill switch is engaged, when `/resume` or `/arm` is sent, then it is refused.

**Data/Kite dependency.** Kite orders, GTT and positions APIs.
**Priority hint.** Must.

### FR-TG-026 Auto-execution arming rules
**Requirement.**
- `/disarm` shall always be allowed (with confirmation).
- `/arm` shall be refused unless `allow_remote_arming` is true in Settings (default false; changing it is desktop-only and needs re-entering the app's confirmation).
- When allowed, `/arm` additionally requires: static-IP check passing, a valid Kite session, kill switch off, inside market hours. The user must reply with a one-time 4-digit code shown in the prompt within 60 s.

Every arm and disarm shall be audit-logged and notified.

**Acceptance criteria.**
- Given the default settings, when `/arm` is sent, then the reply is "Remote arming is disabled; arm from the desktop" and nothing changes.
- Given remote arming is allowed and the static-IP check fails, then arming is refused with the reason.

**Data/Kite dependency.** Static-IP check, `GET /user/profile`.
**Priority hint.** Must.

### FR-TG-027 Paper/live labelling
**Requirement.** Every trade-related message and every confirmation prompt shall carry a prominent `PAPER` or `LIVE` tag. Paper actions shall never call Kite order endpoints.

**Acceptance criteria.**
- Given a paper deployment, when its exit is confirmed from Telegram, then the journal records a simulated fill and no Kite order call is logged.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-028 Notification catalogue and loudness levels
**Requirement.** Each notification type in §F3 shall have its own level: **on** (sound), **silent** (`disable_notification`) or **off**. `exit` and `exit_fill` shall also be configurable per exit reason, with a "default for other reasons" entry. These types are forced on and cannot be set below **on**: `exit_cancel`, `state_attention`, `kill_switch`, `error`, `session` expiry.

| Defaults | Types |
|---|---|
| on | `signal`, `entry_fill`, `exit_fill`, `risk_guard*`, `warning`, `session` |
| silent | `entry`, `exit`, `state_change`, `backtest_done`, `daily_digest` |
| off | `strategy_msg` |
| per scanner | `scanner_alert` |

**Acceptance criteria.**
- Given `exit` = {stop_loss: on, default: silent}, when a target exit occurs, then it is sent silently, while a stop-loss exit is sent with sound.
- Given `kill_switch` is set to off in a hand-edited config, then the app treats it as on and logs a warning.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-029 Message formatting
**Requirement.** Messages shall use Telegram HTML parse mode, with all dynamic text escaped. Formatting rules:
- times in IST (`HH:MM:SS`, plus a date when not today);
- money as ₹ with Indian digit grouping;
- quantities as integers (lots shown for futures);
- instruments as `EXCHANGE:TRADINGSYMBOL`;
- a first line that states the event type and deployment id.

Output over 4096 characters shall be split at line boundaries, and tables shall go in `<pre>` blocks.

**Acceptance criteria.**
- Given a strategy name containing `<` or `&`, then the message renders correctly with no parse error.
- Given ₹1234567.5, then it is shown as ₹12,34,567.50.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-030 Outbound rate limiting and retries
**Requirement.** The sender shall:
- enforce about 1 message/s per private chat and at most 20/min per group, with bursts allowed up to Telegram's tolerance;
- honour `retry_after` on HTTP 429;
- retry network errors up to 3 times with back-off (1 s, 2 s, 4 s);
- drop a message after that and log it.

When the queue backs up, non-critical messages of the same type and deployment (e.g. several `scanner_alert`) shall be **coalesced** into one summary. Critical types (always-on list in FR-TG-028) are never coalesced or dropped before 3 retries.

**Acceptance criteria.**
- Given 50 scanner alerts within 10 s to a group, then no 429 loop occurs and the user receives at most 20 messages that minute, including a coalesced summary.
- Given a 429 with `retry_after`=5, then the next send waits at least 5 s.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-031 Non-blocking, decoupled notification pipeline
**Requirement.**
- **Typed events.** The engine shall publish typed notification events (Go structs per type) to a notifier bus. Telegram, in-app inbox and OS notifications are independent subscribers.
- **Telegram goroutine.** The Telegram subscriber shall consume from a **bounded** buffered channel on its own goroutine.
- **Overflow.** A full channel shall never block the engine. The oldest non-critical message is dropped and counted.
- **Commands.** Inbound commands shall call a transport-neutral command service, the same one the Wails UI uses, so business rules live in one place.
- **Shutdown.** The queue gets 5 s to drain.

**Acceptance criteria.**
- Given Telegram is blocked (network hang), when 10,000 ticks and 200 events occur, then engine tick latency is unchanged and the queue-drop counter is visible in `/health` and in the app.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-032 Inbound command rate limiting
**Requirement.** Inbound commands shall be limited to 20 per minute per authorized user. Excess commands get a single "slow down" reply per minute and are not executed. O-kind commands are limited to 5 confirmations per minute.

**Acceptance criteria.**
- Given 25 `/status` commands within a minute, then 20 are answered and one "slow down" message is sent.

**Data/Kite dependency.** Protects the Kite REST budget.
**Priority hint.** Should.

### FR-TG-033 Daily login reminder and session notices
**Requirement.** While the app is running, the bot shall:
- send a `session` reminder at a configurable time (default 08:45 IST on trading days) if there is no valid Kite session. It states that login must be done on the desktop and that deployments stay idle until then;
- notify at 06:00 IST that the token has expired, if the app was running overnight;
- notify on a successful login, and on any `TokenException` during the day.

Telegram shall never offer or accept credentials, TOTP or `request_token`.

**Acceptance criteria.**
- Given no login by 08:45 on a trading day, then one reminder is sent, and another at 09:10 if still not logged in.
- Given a holiday per the exchange calendar, then no reminder is sent.

**Data/Kite dependency.** Kite session state; trading-holiday calendar.
**Priority hint.** Must.

### FR-TG-034 Scanner and price alerts to Telegram
**Requirement.** Each scanner alert and price alert shall have a "send to Telegram" toggle (default on once Telegram is enabled). A message shall include scanner name, instrument, timeframe, trigger candle time and price.
- A repeated trigger for the same scanner and instrument within one candle is sent once.
- More than N alerts (default 10) from one scan run are sent as one summary message listing instruments.

**Acceptance criteria.**
- Given one scan run matching 25 instruments, then a single summary message lists all 25, chunked if over 4096 characters.

**Data/Kite dependency.** Scanner engine and ticks.
**Priority hint.** Should.

### FR-TG-035 Strategy custom messages
**Requirement.** Strategies may define a "notify" action with a text template. It is delivered as `strategy_msg` (default off), deduplicated per deployment per candle, and limited to 500 characters.

**Acceptance criteria.**
- Given the same notify action firing on every tick within one candle, then only one message is sent for that candle.

**Data/Kite dependency.** Engine.
**Priority hint.** Could.

### FR-TG-036 End-of-day digest
**Requirement.** At a configurable time (default 15:45 IST on trading days), the bot shall send `daily_digest`, which `/digest` also returns on demand:
- realised and unrealised P&L (live and paper separately);
- trades count and win rate;
- deployments by state;
- open carry-forward positions with their GTTs;
- warnings of the day;
- a reminder that the token expires at 06:00.

**Acceptance criteria.**
- Given CNC positions held overnight, then the digest lists them with their active GTT triggers.

**Data/Kite dependency.** Journal, Kite positions and GTT.
**Priority hint.** Could.

### FR-TG-037 Audit log of remote actions
**Requirement.** Every Telegram-originated S or O action shall be recorded in the app's audit log and shown in the desktop activity view, flagged "via Telegram". This covers the request, the confirmation or expiry, and the execution result. Each record holds timestamp, user id, command, parameters, outcome and Kite order ids.

**Acceptance criteria.**
- Given an exit confirmed from Telegram, then the desktop activity view shows "Exit #12 via Telegram by <user> — order 2510…".

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-038 Error handling for commands
**Requirement.** A failing command shall reply with a one-line, user-safe error. It shall not include stack traces or secrets. Full details go to the log. A panic in a handler shall be recovered and must not stop polling or the engine. Validation errors (bad id, bad number) reply with usage help.

**Acceptance criteria.**
- Given a handler panics, then the user receives "Internal error — see app log", polling continues, and the next command works.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-039 Redaction
**Requirement.** No Telegram message shall contain any of the following:
- API key or secret;
- access token;
- bot token;
- full Kite user id (mask it as `AB***12`);
- full account balances in group chats when "hide amounts in groups" is on (default on for groups).

**Acceptance criteria.**
- Given a group chat with hide-amounts on, when `/funds` is sent, then amounts show as "•••" with only percentages.

**Data/Kite dependency.** None.
**Priority hint.** Should.

### FR-TG-040 Reconcile command
**Requirement.** `/reconcile <id>` shall re-read the deployment's orders, positions and GTTs from Kite and update local state. Any inconsistency moves the deployment to ATTENTION; the app does not guess. It shall report what changed.

**Acceptance criteria.**
- Given a position manually closed in Kite, when `/reconcile <id>` is sent, then the deployment's position is set to flat, the orphan GTT is reported, and the deployment moves to ATTENTION.

**Data/Kite dependency.** `GET /orders`, `GET /portfolio/positions`, `GET /gtt/triggers`.
**Priority hint.** Could.

### FR-TG-041 Go Telegram library
**Requirement.** The integration shall use **`github.com/go-telegram/bot`** (MIT, v1.27.0 or later at implementation time, Bot API 10.3, no transitive dependencies). It is used with long polling, context-based cancellation tied to app shutdown, and callback-query routing. The library version and licence shall be recorded in the third-party notices.

**Acceptance criteria.**
- Given `go mod graph`, then the Telegram integration adds no transitive modules beyond the library itself.
- Given app shutdown, then the polling goroutine exits within the context deadline.

**Data/Kite dependency.** None.
**Priority hint.** Must.

### FR-TG-042 No discretionary manual entry from Telegram (v1)
**Requirement.** v1 shall not provide a freqtrade-style "force entry" (buy or short any instrument from chat). New positions may only originate from deployed strategies, via signal confirmation (FR-TG-021) or armed auto-exec.

**Acceptance criteria.**
- Given `/buy INFY`, then the reply is "Not supported — use the desktop".

**Data/Kite dependency.** None.
**Priority hint.** Won't (v1).

---

## H. Open questions

1. **Kill-switch default.** Should the default be "square off all managed positions" or "cancel entries and leave GTT/SL in place"? The answer changes the risk of `/kill`.
2. **Static IP vs Telegram confirmations.** Is it acceptable that a Telegram-confirmed signal is placed from the desktop's static IP, even when the user is away from it? This is technically identical to an in-app click, but the user may expect "confirm from phone" to work while the desktop sleeps. It does not (FR-TG-008).
3. **Telegram vs auto-exec.** Should Telegram confirmation also count as "user confirmation" for SEBI retail-algo framing? A07 suggests that ≤10 OPS own-key usage needs no registration either way; legal confirmation is still pending (A07 open items).
4. **Keyring library.** Which Go keyring library to use for FR-TG-002 on Linux (Secret Service), and the fallback when no keyring daemon runs (e.g. a minimal WM)? UNVERIFIED.
5. **Wider audience.** Is a multi-user group setup worth supporting at all for a single-user app, or should we restrict to private chat only? That would simplify FR-TG-004/006/039.
