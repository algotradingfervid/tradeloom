# 07 — Platform research: Kite Connect, SEBI retail-algo rules, gokiteconnect, Wails v2, data sources

Researched 2026-10-11. Sources are primary (kite.trade docs, Zerodha support, SEBI/NSE circular PDFs, the gokiteconnect and wails git repos, pkg.go.dev, niftyindices/NSE sites). Where the only evidence is a Zerodha staff post on the Kite developer forum, it is labelled **(forum)**. Anything I could not confirm is labelled **UNVERIFIED**.

---

## A. Kite Connect v3

### A1. Plans and pricing

| Plan | Price | Includes | Excludes |
|---|---|---|---|
| **Personal** | Free | Orders, GTT, alerts, margin calculation, portfolio | "No live market data or historical data" |
| **Connect** | ₹500/month per API key (app) | Everything above, plus realtime WebSocket streaming and historical candles. Historical data is "included at no additional cost" | — |

- Pricing page: "Full suite of APIs with realtime WebSocket streaming and historical candle data for ₹ 500 / month". [zerodha.com/products/api](https://www.zerodha.com/products/api)
- Plan contents, ₹500 per API key, and historical data included with Connect: [Kite Connect API FAQs](https://support.zerodha.com/category/trading-and-markets/general-kite/kite-api/articles/kite-connect-api-faqs)
- Price history: an April 2025 Zerodha update made the execution APIs free and charged ₹2000 for data ([Z-Connect, 24 Apr 2025](https://zerodha.com/z-connect/?p=411859)). The current official pages show ₹500, so the price has dropped since then.
- **What this means:** a Streak-like app (scanners, charts, backtests, live signals) needs the **Connect** plan. The free Personal plan can only place and manage orders and read the portfolio.

### A2. Login and session

- **Flow:**
  1. Send the user to the Kite login URL with `api_key`.
  2. Kite redirects to the registered redirect URL with `request_token`. It is single-use and lasts "only a few minutes".
  3. `POST /session/token` with `api_key`, `request_token` and `checksum = SHA-256(api_key + request_token + api_secret)`.
  4. Send the header `Authorization: token api_key:access_token` on every request.

  [kite.trade/docs/connect/v3/user](https://kite.trade/docs/connect/v3/user/)
- **Expiry:** the access token "'ll expire at `6 AM` on the next day (regulatory requirement)". It also ends on logout (`DELETE /session/token`) or a master logout from Kite Web. (same source)
- **Refresh token:** "only available to certain approved platforms". A personal app cannot get one. (same source)
- **Automating TOTP login:**
  - The docs I read have no written ToS clause against it. Zerodha staff (Nivas, 2025) wrote: "As per exchange requirements, users must log in manually at least once each day… we do not recommend automating the Kite Connect login flow" [(forum)](https://kite.trade/forum/discussion/comment/52713/).
  - NSE's implementation standards say "All API sessions shall be compulsorily logged out every day before the start of the next trading day" ([NSE/INVG/67858](https://nsearchives.nseindia.com/content/circulars/INVG67858.pdf)).
  - SEBI requires brokers to "authenticate access to API through two factor authentication" ([SEBI circular 4 Feb 2025](https://www.sebi.gov.in/sebi_data/attachdocs/feb-2025/1738665456458.pdf)).
  - **Design rule:** do a manual, browser-based login once per day. Do not store the TOTP secret or automate the login.
- **Desktop redirect handling:** the redirect URL is registered per app on the developer console. A local app can register `http://127.0.0.1:<port>/callback` and run a loopback listener. **UNVERIFIED:** I did not confirm that Kite accepts loopback/http redirect URLs. Community practice suggests it does, but no primary source confirms it.

### A3. Rate limits ([exceptions page](https://kite.trade/docs/connect/v3/exceptions/))

| Endpoint | Limit |
|---|---|
| Quote (`/quote*`) | 1 req/s |
| Historical candles | 3 req/s |
| Order placement | 10 req/s |
| All other endpoints | 10 req/s |

- **Order-level limits:** 400 orders/min, 5,000 orders/day per user/API key, and 25 modifications per order. (same source)
- **Order-rate scope:** the 10 OPS limit applies "per client (trading) account", not per app. Calls over the limit get HTTP 429, and blocked or invalid requests also count. [Zerodha FAQ](https://support.zerodha.com/category/trading-and-markets/general-kite/kite-api/articles/kite-connect-api-faqs)

### A4. Historical candles ([docs](https://kite.trade/docs/connect/v3/historical/))

- **Endpoint:** `GET /instruments/historical/:instrument_token/:interval?from&to&continuous&oi`
- **Intervals:** `minute, 3minute, 5minute, 10minute, 15minute, 30minute, 60minute, day`
- **Candle format:** `[timestamp, open, high, low, close, volume(, oi)]`
- **`continuous=1`:** NFO/MCX futures only, and **day candles only**. It stitches together expired contracts behind a live contract's token.
- **`oi=1`:** adds open interest as a 7th field.
- **Max days per request:** the official docs do not list these. The figures below are the API's own error strings, quoted on the forum in June 2024 [(forum)](https://kite.trade/forum/discussion/comment/50147/). Older posts give smaller values, so detect the error at runtime and adapt.

  | Interval | Max days per request |
  |---|---|
  | minute | 60 |
  | 3/5/10minute | 100 |
  | 15/30minute | 200 |
  | 60minute | 400 |
  | day | 2000 |

- **Lookback** [(forum, staff sujith, Jun 2024)](https://kite.trade/forum/discussion/comment/50147/):
  - Minute-level data "starts somewhere around early 2015".
  - For day candles "there is no fixed date"; some NSE stocks go back to the late 1990s.
  - Coverage varies by instrument.
- **No bulk fetch:** staff advise fetching once and storing the data locally ([forum](https://kite.trade/forum/discussion/8899/rate-limit-of-kite-connect-historical-data-api-and-lookback)).
- **Expired F&O contracts:** **UNVERIFIED** whether intraday data for expired option contracts can be fetched. Expired tokens drop out of the instruments dump.
- **Backfill cost at 3 req/s** (my arithmetic from the limits above):

  | Universe and data | Requests | Time |
  |---|---|---|
  | NIFTY 500, 10 years of 1-minute | ≈ 500 × 61 ≈ 30,500 | ≈ 2.8 h |
  | NIFTY 500, about 5.5 years of daily (2,000 days) | 500 | ≈ 3 min |

### A5. Instruments, quotes, LTP, OHLC ([market-quotes](https://kite.trade/docs/connect/v3/market-quotes/))

- **Instruments dump:**
  - Endpoints: `GET /instruments` or `/instruments/:exchange`.
  - Format: gzipped CSV with columns `instrument_token, exchange_token, tradingsymbol, name, last_price, expiry, strike, tick_size, lot_size, instrument_type, segment, exchange`.
  - It is generated "once everyday". The docs suggest fetching it around 08:30 and caching it.
- **Quote endpoints:**

  | Endpoint | Returns | Max instruments per call |
  |---|---|---|
  | `GET /quote` | Full snapshot, including depth | 500 |
  | `GET /quote/ohlc` | OHLC + last price | 1000 |
  | `GET /quote/ltp` | Last price | 1000 |

  All three share the 1 req/s quote limit.

### A6. WebSocket ticker ([websocket](https://kite.trade/docs/connect/v3/websocket/))

- **Modes:**

  | Mode | Packet size | Contents |
  |---|---|---|
  | `ltp` | 8 bytes | Last price |
  | `quote` | 44 bytes | Price fields, no depth |
  | `full` | 184 bytes | Includes 5-level depth |

- **Capacity:** "up to **3000** instruments on a single WebSocket connection", and "upto 3 websocket connections" per API key, so at most **9,000 instruments** stream live.
- **Order updates:** come as JSON text frames (`type: order | error | message`) on the same socket. The docs recommend this for individual developers instead of HTTP postbacks.

### A7. Orders ([orders](https://kite.trade/docs/connect/v3/orders/))

- **Varieties:** `regular, amo, co, iceberg, auction`. (gokiteconnect still defines a `bo` constant, but BO is not in the current docs.)
- **Order types:** `MARKET, LIMIT, SL, SL-M`
- **Products:** `CNC, NRML, MIS, MTF`
- **Validity:** `DAY, IOC, TTL` (set `validity_ttl` in minutes)
- **Iceberg:** 2–50 legs.
- **`market_protection`:** `0<x≤100` sets a percentage, `-1` sets it automatically. It applies to MARKET and SL-M orders.
  - Zerodha FAQ: "Orders with a market protection value of 0 are treated as unprotected market orders and will be rejected" ([FAQ](https://support.zerodha.com/category/trading-and-markets/general-kite/kite-api/articles/kite-connect-api-faqs)).
  - **Every API market order must set it.**
- **`autoslice=true`:** splits orders above the freeze quantity, up to 10 slices.
- **`tag`:** up to 20 alphanumeric characters. Use it to tag strategy or run IDs.
- **MCX:** IOC is not supported in the algo segment. (FAQ)

### A8. GTT ([gtt](https://kite.trade/docs/connect/v3/gtt/))

- **Types:** `single` and `two-leg` (OCO).
- **Endpoints:** `POST/GET /gtt/triggers`, plus `GET/PUT/DELETE /gtt/triggers/:id`.
- **Orders:** the examples show only LIMIT orders. A modify resends the full parameter set.
- **Expiry:** each GTT has an `expires_at` field. The sample responses show about one year.
- **Count limit:** the docs state no limit on active GTTs (**UNVERIFIED**).

### A8b. Alerts ([alerts](https://kite.trade/docs/connect/v3/alerts/))

- **Types:** `simple` and `ato` (Alert Triggers Order, which places a basket when triggered).
- **Limit:** up to **500 active alerts** per user.
- **Conditions:** operators `<=, >=, <, >, ==` on an attribute such as `LastTradedPrice`, compared with a constant or with another instrument.
- **Use:** these run on Zerodha's servers, so they are the only built-in way to act on a price when the desktop app is off. They are limited to simple price conditions.

### A9. Margins and charges ([margins](https://kite.trade/docs/connect/v3/margins/))

- **`POST /margins/orders`:** margin per order. Fields: span, exposure, option_premium, var, leverage, charges, total, and more.
- **`POST /margins/basket`:**
  - Parameters: `consider_positions`, `mode=compact`.
  - Returns `initial`, `final` (with spread benefit), and per-order margins.
  - The docs say to use the per-order `charges`, not the top-level block.
- **`POST /charges/orders`:** a "virtual contract note". Returns brokerage, STT/CTT, exchange turnover charge, SEBI fee, stamp duty and GST for each order. Use it for realistic P&L in paper trading and backtest reports, by submitting synthetic orders.

### A10. Postbacks ([postbacks](https://kite.trade/docs/connect/v3/postbacks/))

- **Mechanism:** HTTP POST of raw JSON to the app's registered `postback_url` on order status COMPLETE, CANCEL(LED), REJECTED or UPDATE. Verify the checksum as `SHA-256(order_id + order_timestamp + api_secret)`.
- **Intended for** "platforms… where a single api_key will place orders for multiple users".
- **Why we won't use it:** a desktop app with no public HTTPS endpoint cannot receive postbacks. **Use WebSocket order updates instead.**

### A11. Portfolio and funds

- **Portfolio** ([portfolio](https://kite.trade/docs/connect/v3/portfolio/)):
  - `GET /portfolio/holdings`
  - `GET /portfolio/positions`, which returns `net` and `day`
  - `PUT /portfolio/positions` to convert a position's product
  - `GET /portfolio/holdings/auctions`
- **Funds** ([user](https://kite.trade/docs/connect/v3/user/)):
  - `GET /user/margins` and `/user/margins/:segment` (equity or commodity).
  - Returns `net`, `available.{cash, opening_balance, live_balance, collateral, intraday_payin, adhoc_margin}` and `utilised.{span, exposure, option_premium, m2m_*, …}`.

### A12. Static IP (Zerodha implementation) ([FAQ](https://support.zerodha.com/category/trading-and-markets/general-kite/kite-api/articles/kite-connect-api-faqs))

- Static IP is **mandatory for order placement from 1 Apr 2025**. Other endpoints work from any IP.
- **Registration:** up to 2 IPs (primary and secondary), IPv4 or IPv6. They are set at developer-account level and apply to all apps.
- **Changes:** at most one per calendar week.
- **Sharing:** only within immediate family.
- **Scope:** "required regardless of order rate".

---

## B. SEBI retail algo framework (in force for all brokers from 1 Apr 2026)

**Sources:**
- SEBI circular SEBI/HO/MIRSD/MIRSD-PoD/P/CIR/2025/0000013, 4 Feb 2025 ([PDF](https://www.sebi.gov.in/sebi_data/attachdocs/feb-2025/1738665456458.pdf))
- SEBI extension circular CIR/2025/132, 30 Sep 2025 ([PDF](https://www.sebi.gov.in/sebi_data/attachdocs/sep-2025/1759232056254.pdf))
- NSE implementation standards NSE/INVG/67858, 5 May 2025 ([PDF](https://nsearchives.nseindia.com/content/circulars/INVG67858.pdf))

**Timeline:**
- The circular was due to apply from 1 Aug 2025. That was extended to 1 Oct 2025 (circular of 29 Jul 2025).
- Brokers who were ready went live on 1 Oct 2025. The rest followed a glide path:
  - 31 Oct 2025: apply to register products
  - 30 Nov 2025: complete registration
  - 3 Jan 2026: take part in a mock session
- "W.e.f. April 01, 2026, algo framework … will be applicable for all stock brokers." (Sep 2025 circular, para 8)
- I found no further extension after April 2026. A broker notice from March 2026 ([FYERS](https://fyers.in/notice-board/new-sebi-framework-for-retail-algo-trading-from-april-01-2026/)) confirms the 1 Apr 2026 date.

**Key rules:**

1. **Self-built algos are allowed:**
   - SEBI: "Algos developed by tech-savvy retail investors themselves… shall also be registered with the Exchange, through their broker, **only if they cross the specified order per second threshold**".
   - A registered algo may be used for **family only** (self, spouse, dependent children, dependent parents). (SEBI Feb 2025, 5.I.c)
2. **Threshold:**
   - NSE: "Threshold Order Per Second (TOPS) is initially set at not exceeding 10 orders per second per exchange". At or below that, "the client will not be required to register". Such orders are tagged with a **generic algo ID provided by the Exchange**.
   - Above 10 OPS, the algo must be registered with each exchange and gets a unique ID.
   - Brokers must reject orders above the threshold and may set lower per-client limits. (NSE 67858, §B/§F)
3. **Static IP:**
   - Mandatory for API access for client-generated algos. The IP must be the client's own.
   - Primary plus optional secondary IP, with at most one change per calendar week.
   - An IP can map to only one client, except within a family.
   - With multiple API keys, non-registered algos must run through one predefined key. (NSE 67858, §A)
4. **Daily logout:** "All API sessions shall be compulsorily logged out every day before the start of the next trading day" (NSE §A.8). Brokers must use OAuth-only authentication and 2FA (SEBI 5.I.d).
5. **No open APIs:** access must use "a unique vendor client specific API key and static IP whitelisted by the broker" (SEBI 5.I.d).
6. **Algo providers:**
   - Third-party vendors offering algos through broker APIs must be **empanelled with exchanges**. Brokers act as principal and vendors as their agent.
   - "Black box" algos (logic not disclosed) need Research Analyst registration. (SEBI 5.I.a, III, V)
7. **Market orders:**
   - Zerodha rejects API market orders without market protection (A7).
   - FYERS likewise converts market orders to market-price-protection orders (MPP).
   - **UNVERIFIED** whether this is an exchange-wide rule.

**Effect on this app:**
- A local app using the trader's **own** API key, from their **own** static IP, at ≤10 OPS, is a permitted unregistered client-generated algo.
- Zerodha's 10 OPS per account cap already enforces the threshold.
- **Open legal question (UNVERIFIED, needs advice):** if the app is distributed to other people, does the distributor become an "algo provider" needing exchange empanelment? Each user would still use their own key and IP. The circulars define algo providers as those "providing the facility to place algo orders with Brokers through API". A white-box tool that the user configures is arguably different from a vendor service, but I found no ruling either way.

---

## C. gokiteconnect (verified by cloning [github.com/zerodha/gokiteconnect](https://github.com/zerodha/gokiteconnect))

- **Latest release:** `v4.4.3`, tagged 2026-09-15. The last commit on master is the same day: "fix(ticker): add NCO segment mapping…".
  - Earlier tags: v4.4.2 (2026-07-08), v4.4.1 (2026-06-29), v4.4.0 (2026-03-28).
  - Module path: `github.com/zerodha/gokiteconnect/v4`.
- **Maintenance:** actively maintained by Zerodha staff.
- **Go versions:** `go.mod` declares `go 1.14`. CI (`.github/workflows/go-test.yml`) tests Go 1.21, 1.22, 1.23 and 1.24 on ubuntu-latest. Newer Go versions are not in the CI matrix (**UNVERIFIED**, though very likely fine).
- **Coverage** (methods on `*Client`):

  | Area | Methods |
  |---|---|
  | Session | `GetLoginURL`, `GenerateSession`, `InvalidateAccessToken`, `RenewAccessToken` (refresh token, approved platforms only) |
  | Market data | `GetInstruments`, `GetInstrumentsByExchange`, `GetQuote`, `GetLTP`, `GetOHLC` |
  | Historical | `GetHistoricalData(token, interval, from, to, continuous bool, oi bool)` |
  | Orders | `PlaceOrder`, `ModifyOrder`, `CancelOrder`, `ExitOrder`, `GetOrders`, `GetOrderHistory`, `GetTrades`, `GetOrderTrades` |
  | GTT | `PlaceGTT`, `ModifyGTT`, `DeleteGTT`, `GetGTT`, `GetGTTs` |
  | Alerts | `CreateAlert`, `GetAlerts`, `GetAlert`, `ModifyAlert`, `DeleteAlerts`, `GetAlertHistory` |
  | Margins and charges | `GetOrderMargins`, `GetBasketMargins`, `GetOrderCharges` (virtual contract note) |
  | Portfolio and funds | `GetHoldings`, `GetHoldingsSummary`, `GetHoldingsCompact`, `GetPositions`, `ConvertPosition`, `GetAuctionInstruments`, `GetUserMargins`, `GetUserSegmentMargins` |
  | Other | Mutual funds |

- **`OrderParams` fields:** includes `MarketProtection`, `Autoslice`, `ValidityTTL`, `Iceberg*`, `AuctionNumber`, `Tag`, and a newer **`AlgoID`** field (`algo_id`, added in the 2026-06-29 PR "feat-order-algo-id"). The public Kite orders doc page I read does not describe `algo_id`, so check its semantics before use.
- **Ticker:** the `ticker` package provides `ModeLTP`, `ModeQuote` and `ModeFull`.
  - Methods: `Subscribe`, `Unsubscribe`, `SetMode`, `Resubscribe`, `OnTick`, `OnOrderUpdate`, `OnReconnect`.
  - Auto-reconnect is built in (`SetAutoReconnect`, `SetReconnectMaxRetries`, `SetReconnectMaxDelay`), and `ServeWithContext` is available.
  - Each `Ticker` is one connection, so open 3 tickers to reach 9,000 instruments.

---

## D. Wails v2 on Linux, packaging, and embedded DB

Verified from the wails repo at tag `v2.16.0` (website/docs and source).

- **Release state:**
  - Latest v2 is **v2.16.0 (2026-09-14)**; v2.15.0 was 2026-08-17. The changelog shows v2 actively maintained.
  - **v3 is still pre-release.** The latest tag is `v3.0.0-beta.28` (2026-10-05), and there is no v3.0.0 final tag. (`git ls-remote --tags https://github.com/wailsapp/wails`)
- **Requirements:**
  - Go 1.21+ and npm (Node 15+). Linux needs gcc, libgtk-3 and libwebkit2gtk.
  - Supported platforms: Linux amd64/arm64 (`website/docs/gettingstarted/installation.mdx`).
- **webkit2gtk 4.0 vs 4.1:**
  - The source uses `#cgo !webkit2_41 pkg-config: webkit2gtk-4.0` and `#cgo webkit2_41 pkg-config: webkit2gtk-4.1` (`v2/internal/frontend/desktop/linux/frontend.go:8-9`).
  - Docs: "If you are using a Linux distribution that does not have webkit2gtk-4.0 (such as Ubuntu 24.04), you will need to add `-tags webkit2_41`" (`gettingstarted/building.mdx`).
  - **Recommendation:** build with `-tags webkit2_41` against `libwebkit2gtk-4.1-dev`. Ubuntu 24.04 ships it ([packages.ubuntu.com noble](https://packages.ubuntu.com/en/noble/gir1.2-webkit2-4.1)), as do Fedora and Arch.
  - The same binary then needs the 4.1 runtime on the target system. I did not check which current distros still ship 4.0 (**UNVERIFIED**).
- **CGO:** the Linux frontend is cgo plus GTK/WebKit, so **every Linux build needs CGO and Linux headers**.
- **Cross-compiling from macOS to Linux:**
  - Not practical natively.
  - The v2 docs suggest only per-OS GitHub Actions runners, via the `dAppServer/wails-build-action` example (`guides/crossplatform-build.mdx`).
  - **Recommendation:** build in a Linux Docker container (Ubuntu 24.04 with `libgtk-3-dev libwebkit2gtk-4.1-dev`) or in Linux CI.
  - v3 automates this with `wails3 task setup:docker` ("From macOS or Windows, you'll need to set up Docker first"), but that is beta-only.
- **Packaging:**
  - v2's `wails build` produces a bare binary. Its CLI has no .deb or AppImage option (only `-nsis` for Windows; checked `v2/cmd/wails/flags/build.go`).
  - **Options for v2:** wrap the binary yourself with nfpm (.deb/.rpm) and appimagetool or linuxdeploy (AppImage). These are standard tools, not Wails features.
  - v3 has `wails3 task linux:create:{appimage,deb,rpm}` using nfpm (v3 `guides/build/linux.md`).
- **Wails v3 notes:**
  - Defaults to **GTK4 / WebKitGTK 6.0**.
  - GTK3/WebKit2GTK 4.1 is available via `-tags gtk3` "through the v3.0.x line", and is "scheduled for removal in v3.1".
  - **Recommendation:** stay on v2 for now and keep the frontend framework-agnostic so a later v3 port is cheap.
- **Embedded DB options:**

  | Driver | CGO | Notes |
  |---|---|---|
  | `modernc.org/sqlite` | No | "CGo-free port"; v1.60.1 (2026-09-29), SQLite 3.53.4, linux/amd64+arm64 supported ([pkg.go.dev](https://pkg.go.dev/modernc.org/sqlite)) |
  | `github.com/ncruces/go-sqlite3` | No | Wasm build of SQLite translated to Go, with higher memory use ([repo](https://github.com/ncruces/go-sqlite3)) |
  | `mattn/go-sqlite3` | Yes | Standard cgo driver |
  | DuckDB `github.com/duckdb/duckdb-go` | Yes | Successor to `marcboeker/go-duckdb` from v2.5.0. Prebuilt static libs for linux amd64/arm64, darwin amd64/arm64 and windows amd64; DuckDB v1.5.6 ([repo](https://github.com/duckdb/duckdb-go)) |

  - **Effect on cross-compiling:** none in practice, because Wails Linux already forces CGO and a Linux toolchain. Choose on merits.
  - **Suggestion:** use pure-Go SQLite (modernc) for app state, orders and settings. Consider DuckDB, or a Parquet/columnar layout, only if candle analytics over tens of millions of rows needs it. Minute bars for 500 stocks over 10 years come to about 500 × 375 × 250 × 10 ≈ 470M rows, which is large for SQLite but fine for DuckDB.

---

## E. Free supplementary NSE data

All URLs below were probed with curl on 2026-10-11 and returned HTTP 200 unless noted.

- **Index constituents (niftyindices.com):**
  - URLs: `https://www.niftyindices.com/IndexConstituent/ind_nifty50list.csv`, `ind_nifty100list.csv`, `ind_nifty200list.csv`, `ind_nifty500list.csv`, `ind_niftybanklist.csv`.
  - Columns: `Company Name, Industry, Symbol, Series, ISIN Code`. Row counts were 50, 100, 200, about 500 and 14 at fetch time.
  - **Current lists only.** I found no point-in-time history here, so backtests that use today's list carry survivorship bias.
- **CM bhavcopy, current format (UDiFF):**
  - URL: `https://nsearchives.nseindia.com/content/cm/BhavCopy_NSE_CM_0_0_0_YYYYMMDD_F_0000.csv.zip`.
  - Columns include `TradDt, TckrSymb, SctySrs, ISIN, OpnPric, HghPric, LwPric, ClsPric, PrvsClsgPric, TtlTradgVol, TtlTrfVal, TtlNbOfTxsExctd…`.
  - Available for 2024-01-01; a 2020 date returned 404.
  - F&O equivalent: `/content/fo/BhavCopy_NSE_FO_…` (includes OI).
- **CM bhavcopy, legacy format:**
  - URL: `https://nsearchives.nseindia.com/content/historical/EQUITIES/YYYY/MON/cmDDMONYYYYbhav.csv.zip`.
  - 200 for 2000-01-03 through 2024-07-05; 404 from 2024-07-08.
  - Matches NSE's note that legacy reports were discontinued from 8 Jul 2024 under circular 62424 dated 12 Jun 2024 ([NSE all-reports](https://www.nseindia.com/all-reports)). **A historical EOD loader needs both parsers.**
- **Delivery data:** `https://nsearchives.nseindia.com/products/content/sec_bhavdata_full_DDMMYYYY.csv`, with columns `SYMBOL, SERIES, DATE1, PREV_CLOSE, OPEN/HIGH/LOW/LAST/CLOSE/AVG_PRICE, TTL_TRD_QNTY, TURNOVER_LACS, NO_OF_TRADES, DELIV_QTY, DELIV_PER`.
- **Not adjusted:** bhavcopies are raw closes, with no split or bonus adjustment. Corporate-action adjustment has to be built separately. **UNVERIFIED:** whether Kite day candles are adjusted. Kite docs don't say.
- **Terms of use:**
  - NSE: "User is prohibited to conduct any systematic or automated data collection activities (including scraping, data mining, data extraction and data harvesting) on or in relation to our Website". Content may not be copied or stored "in an electronic retrieval system" without permission ([nseindia.com/static/nse-terms-of-use](https://www.nseindia.com/static/nse-terms-of-use)).
  - niftyindices.com: "for personal, non-commercial use only… You may not conduct any systematic or automated data collection activities (including scraping…) without our express written consent" ([niftyindices.com/terms-of-use](https://niftyindices.com/terms-of-use)).
  - **Implication:** automated bulk download of bhavcopies or constituent CSVs breaches the published terms, even though the files are public. Options:
    1. Have the user trigger downloads manually (user-initiated, personal use).
    2. Import files the user downloaded themselves.
    3. Rely on Kite historical data, which is licensed under the Connect plan, as the primary source. Licensed alternatives include NSE Data & Analytics products (paid).
  - BSE terms were not checked (**UNVERIFIED**).

---

## Constraints that shape the product

1. **Paid data plan required.** Scanners, charts, backtests and live signals all need Kite **Connect at ₹500/month per API key**. The free Personal plan works only for an "order pad plus portfolio" mode.
2. **No always-on, server-side deployment.**
   - Streak runs deployed strategies on its own servers even when the user is offline. A local app works only while the machine is running, online and logged in.
   - The session dies at 06:00 the next day and needs a **manual login every trading day**; automated TOTP login is discouraged by Zerodha and conflicts with exchange 2FA rules.
   - The only server-side fallbacks are Kite **GTT** (single/OCO, limit orders) and **ATO alerts** (500 max, simple price conditions). They can carry exits or stop-losses while the app is closed, but not arbitrary indicator logic.
3. **Static IP needed for auto-trading.**
   - Order placement needs a **registered static IP** (max 2, one change per week). Most home connections need a static IP add-on from the ISP or a static-egress VPN/VPS.
   - Without it, the app can scan and alert but cannot place orders. Expose "signal-only / manual confirm" as a first-class mode.
4. **Order-rate ceiling.** Stay at or under **10 OPS per exchange per account**; above that, exchange algo registration is needed.
   - Hard cap the order throttle at well below 10/s, and respect 400/min, 5,000/day and 25 modifications per order.
   - Basket or multi-symbol deployments must queue orders.
5. **Market orders need protection.** Always send `market_protection` (for example `-1`), or the order is rejected. Avoid IOC on MCX.
6. **Live universe cap: about 9,000 instruments** (3 sockets × 3,000).
   - Enough for NIFTY 500 plus F&O underlyings, but not every NFO option strike at once.
   - Option-chain style screens must subscribe selectively.
   - Polling quotes is limited to 1 req/s (500–1000 instruments per call).
7. **Historical data is rate-limited and chunked.**
   - 3 req/s, with per-interval windows of 60/100/200/400/2000 days. Intraday history goes back only to about 2015.
   - The app needs a local candle store, an incremental backfill job with resume, and progress UI. A first full minute-level backfill of NIFTY 500 takes hours.
   - Continuous futures exist only for **day** candles, so intraday backtests on rolling futures or expired options are limited (**UNVERIFIED** for expired option intraday).
8. **Expect gaps against Streak.** Streak's server-side scanning across the whole market every candle can only be approximated, within the 9k-instrument tick limit, by aggregating candles from ticks locally.
9. **Index membership and survivorship.** Free constituent lists are current-only, and automated scraping of NSE or niftyindices breaches their terms. Backtests over "NIFTY 50 as of date X" are not reproducible with free data. Ship user-imported lists and document the bias.
10. **Charges and brokerage.** Use `/charges/orders` (virtual contract note) and `/margins/basket` for accurate costs and margin checks before deploying, instead of hard-coding fee tables.
11. **Distribution and legal.** Personal use with your own key, IP and family is clearly permitted under the SEBI and NSE rules. **Publishing the app for others** may raise algo-provider empanelment questions (**UNVERIFIED, needs legal review**). Keep strategies white-box and user-authored.
12. **Build pipeline.**
    - Wails v2 on Linux needs CGO, GTK3 and WebKit2GTK. Build with `-tags webkit2_41` in a Linux container or CI; macOS-native cross-compiling is not practical.
    - Package .deb or AppImage with nfpm or appimagetool yourself; v2 has no built-in Linux packaging.
    - Pure-Go SQLite is fine; DuckDB adds no extra cross-build pain because CGO is already required.
