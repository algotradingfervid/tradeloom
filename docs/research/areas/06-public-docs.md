# 06 — Streak public documentation research

Research date: 2026-10-11. Public sources only; no login, no browser extension.

## Read this first: which "Streak" is documented

- **There are two Streak products, and their behaviour differs.** The legacy platform (streak.tech / pro.streak.tech, 2017–2026) was shut down on **6 Feb 2026, 3:30 PM**. Everything now runs on the rebuilt "New Streak (Beta)" at beta.streak.tech (it redirects to www.streak.tech). Strategies and scanners could not be migrated. [staff, 23 Feb 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517?page=6), [staff, 23 Jan 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517), [help](https://help.streak.tech/uzi0iv5zi5ka7f5a5hdjkab0)
- **The rebuild should copy the new platform.** The current help center (help.streak.tech, articles dated Nov 2025 to Sep 2026) is the primary source here. Legacy facts are kept only where they fill gaps, and are marked **[LEGACY]**. Anything older than about 2 years (before Oct 2024) is marked **[OLD, possibly outdated]**.
- **How the help center was read.** help.streak.tech is a client-side Next.js app. Its article text comes from a Strapi CMS (`help-be.streak.tech/api/articles`) using a read token that the public page itself ships. All 143 articles were read through that same public endpoint. Each article's public URL is `https://help.streak.tech/<documentId>`, and those are the links cited below.

---

## 1. Backtesting rules

### Fills and signal timing
- **Signals fill at the next candle's open.** All buy/sell signals execute at the open price of the next candle, regardless of that candle's volume. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **Entries need a fresh transition.** An entry happens only when the entry condition is true on the current candle and was false on the previous one. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **No averaging or pyramiding.** A new position opens only after the previous one has fully exited. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **The displayed trigger time is the signal candle's open time.** A condition met on the 9:20–9:25 candle shows as 9:20 in transaction details, to match broker charts, but fills at the 9:25 open. [staff, 29–30 Dec 2025](https://tradingqna.com/t/streak-operates-on-different-clocks-it-needs-a-fix/189928)
- **All signals use completed candles.** Signals are evaluated on completed-candle OHLC, never on LTP or ticks, in both backtest and live trading. [help](https://help.streak.tech/d6vkbodone0hsip7dbikgu3n)
- **Today's backtest data lags.** Backtest and trade details for the current day update after 9 PM. [help](https://help.streak.tech/wtbu1t4t6frfyaqt6ipmwqny)

### Data range (lookback) per base timeframe
| Base TF | Lookback |
|---|---|
| 1 min | 30 calendar days |
| 3–30 min | 90 calendar days |
| 1 hour | 365 calendar days |
| 1 day | 3 years |

- **Range is per base timeframe.** The table above comes from [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz) and [Advanced Create](https://help.streak.tech/gyazemxw74t8urirmc9c9ifj). The backtest window can sit anywhere inside the lookback.
- **The lookback also limits indicator warm-up.** For example, an EMA(50) on a Monthly MTF condition cannot compute, because there are only 3 years of daily data. [help](https://help.streak.tech/m59fl457tk69kfgrne4guyxh), [help](https://help.streak.tech/d9d88xokyx98p1i7ifnnzrgv)
- **Base timeframes.** 1, 3, 5, 10, 15, 30 min, 1 hr and 1 day. [help](https://help.streak.tech/bveyng9ssjyw5mhdysi9c55z)
- **Weekly and Monthly.** These work only as MTF condition timeframes, never as the base. [help](https://help.streak.tech/qm9s724jvyzdq115l3ce7wuk)
- **[OLD] Lookback used to be longer.** In 2024 the 1-day lookback was 5 years. [Z-Connect comments, Feb 2024](https://zerodha.com/z-connect/?p=368853) An earlier forum answer also gives 5 years for daily. [TradingQnA](https://tradingqna.com/t/query-in-ctb-about-backtest-period/136818)

### Intraday (MIS) square-off
- **Equity MIS.** Generic rule: squared off at the open of the 3:15 PM candle. Chart marker "SO". [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz), [help](https://help.streak.tech/m2ggix0dgfykw0h0nre6faa2)
- **Instrument-specific times (newer, Sep 2026).**
  - F&O stocks under the CAS mechanism: **3:10 PM**. [help](https://help.streak.tech/hzf1m5g2hi1mkxbtc1ehj838)
  - Non-F&O equity: **3:25 PM**. [help](https://help.streak.tech/ui4j1uome0k35bkriyiesfuw)
  - Futures: **3:25 PM**. [help](https://help.streak.tech/yrioxa1d9cjf5qu41f86hcja)
  - Indices: **3:10 PM**. [help](https://help.streak.tech/bjs4w9aho02sxndy3nrb42d7), [summary](https://help.streak.tech/ejnu7smyka178b65wmpkbjpk)
  - 1-hour timeframe on F&O stocks: the **2:15 PM** candle, because after CAS there is no 1-hour candle after 3:15. [help](https://help.streak.tech/aq43z20dypj4jhzedxtsv4bb)
- **Currency MIS.** Squared off at the open of the 4:30 PM candle. The currency segment is not tradable on the new platform. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz), [help](https://help.streak.tech/y2lo4294ydq3neuwdtdegy45)

### Position sizing and capital
- **No margin or leverage.** None is assumed for any order type. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **Capital requirement.** Initial Capital must be ≥ Quantity × Close for every instrument. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **Trade gating.** A trade happens only if `Price × Qty ≤ Initial Capital + cumulative P&L up to the previous trade`. Otherwise the error is "No trades happened because the initial capital is low". [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz), [Error messages](https://help.streak.tech/fszore8axs122z4n7hn7lkfb)
- **Freeze limits ignored.** Exchange quantity freeze limits are not applied in backtests. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **[LEGACY/OLD] Capital is per instrument.** Initial capital and Max Allocation applied to each instrument separately, not split across them, and quantity was roughly max allocation ÷ price. "Risk-based position sizing" also existed. [TradingQnA staff replies](https://tradingqna.com/t/initial-capital-max-allocation-problem/188586), [TradingQnA](https://tradingqna.com/t/how-to-use-max-allocation-parameters/141832). Whether the new platform keeps a Max Allocation field is not stated in the current docs.

### Trading charges
- **What is included.** Brokerage, STT, transaction charges, GST and SEBI fees, taken from the broker's brokerage calculator and applied on **both buy and sell** legs. Stamp duty is not named. [Trading Charges](https://help.streak.tech/yz0rnnn3d6609r0g0xrhb67m)

### Slippage
- **Current docs: no slippage model.** The current docs describe the backtest as "near-ideal … fewer or no slippages". [help](https://help.streak.tech/doe6tejb3u04kl1ervsiobfa)
- **[LEGACY/OLD 2018] Flat slippage.** The legacy platform applied a "flat minimum slippage" and filled SL at the close of the candle after the trigger candle. [TradingQnA staff, 2018](https://tradingqna.com/t/streak-does-not-account-for-slippage/41389)

### SL / TP / TSL evaluation
- **Same-candle check on entry.** SL and TP are checked on the entry candle. If both hit in the same candle, **SL is assumed first**. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **Intra-candle path for TSL.** Longs assume O→L→H→C and shorts assume O→H→L→C. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **P&L evaluation.** "Profit and loss are computed based on Target/Stop Loss percentages and evaluated on the close price of every candle while a position is active." This wording conflicts with the intra-candle path above; see Unverified. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)
- **Exit units.** SL, TP and TSL can be set in **%, points, or absolute/P&L** terms. [FAQ](https://help.streak.tech/wcyaguzrnqavp82hu2d0xxeu), [Equity Strategy](https://help.streak.tech/pfgdsjkva1zd65ika53eyrwk)
- **No conditional TSL.** Dynamic SL modification is not supported (Feb 2026). [staff](https://tradingqna.com/t/meet-the-new-streak-beta/190517)
- **Trade-log trigger codes.** EN, EX, SL, TP, TSL, SO (square-off) and EXP (contract expired). [Transaction Details](https://help.streak.tech/tucjl1gvg3groj0xv1q5cyl9)
- **No corporate-action adjustment.** Raw OHLCV is used, with no adjustment for splits or bonuses. [Backtest Assumptions](https://help.streak.tech/imz40d92sish0tgig1prdfiz)

### Re-entry, trades per day, time window
- **Strategy cycle and entry window.** "Strategy cycle", "Entry Start Time" and "Entry Stop Time" in **Backtest Parameters** set how many entries are allowed per day and the time window for them. [staff, 10 Mar 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517?page=6)
- **[LEGACY] Cycle semantics.** "Daily Strategy Cycle" capped hypothetical trades per day in a backtest. In live deployment, a cycle is how many times the strategy re-arms after an exit. A CNC cycle counted per deployment and an MIS cycle per day. [TradingQnA](https://tradingqna.com/t/it-took-4-trades-today-but-i-selected-only-3-trades-how/139711), [Z-Connect 2018 comments](https://zerodha.com/z-connect/tradezerodha/streak/streak-just-got-a-whole-lot-better)
- **Day-of-week restriction.** Time-based options strategies can be limited to specific weekdays. [help](https://help.streak.tech/bwedmi1in5ihbcblhte3l5t4)

### Holding type, segments, chart types
- **Product types.** MIS (intraday) and CNC. Live MIS strategies stop triggering after 3:20 PM and expire at 3:21 PM. Use CNC to trigger after 3:20. [help](https://help.streak.tech/v9z15qqr7lcdikfzw9cts8mv), [help](https://help.streak.tech/u35mikvpcigwrd72k6bonzmd)
- **Short selling.** Intraday short-selling backtests on equity are supported. [staff, Feb 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517)
- **Options backtesting is not available on the new platform.** Only equities and futures can be backtested; this was confirmed as of 3 Sep 2026. [help](https://help.streak.tech/l9j9k560rvqifmpgvbh1c82l), [staff](https://tradingqna.com/t/meet-the-new-streak-beta/190517?page=6)
- **Chart types: candlestick only.** Heikin-Ashi and Renko are not supported. [help Renko](https://help.streak.tech/pb3019ik1h7z3izmzdqckzu6), [help HA](https://help.streak.tech/lumxubfpkincw89hmstqjl60)
- **[LEGACY] Older chart types.** Candlestick and Heikin-Ashi were available. Renko was on the Ultimate plan in 2018–19 and absent by 2024. [Z-Connect Jan 2024](https://zerodha.com/z-connect/?p=368853)
- **Instruments.** NSE, plus BSE stocks in the BSE 1000 index only. No MCX, currency, AMO or MTF. [help](https://help.streak.tech/exshxnr63rumuuccdtp0f0ax), [help](https://help.streak.tech/m9knflf8pq418jyqb18aq30l), [help](https://help.streak.tech/aw9jfdp2j218picamdzoe0uf), [help](https://help.streak.tech/da7nay9u2kdm1visb5e5zai1)

### Multi-timeframe (MTF) evaluation
- **Evaluation cadence.** The strategy is evaluated on every **base-TF candle close**. Each higher-TF condition uses its **last fully closed** candle. Timeframes are set per condition, not per indicator. [Advanced Create](https://help.streak.tech/gyazemxw74t8urirmc9c9ifj), [help](https://help.streak.tech/bveyng9ssjyw5mhdysi9c55z)
- **Non-multiple timeframes.** Triggers fire at their LCM, for example 3 and 5 min fire every 15 min. [help](https://help.streak.tech/o2icipegxzppazmru8ostoef), [Options trader](https://help.streak.tech/cmun5e4no1to4l7tzyvr3rds)
- **Weekly candles.** A weekly candle counts as complete only when the next week begins. [help](https://help.streak.tech/s0z62rroc7w2y2cf8goicku3)
- **Condition limits.** Basic mode allows 5 entry and 5 exit conditions. Advanced mode allows 10, with AND/OR, Group(), math operators, Period Min/Max, Min/Max, Ceil and Floor. [FAQ](https://help.streak.tech/wcyaguzrnqavp82hu2d0xxeu), [Math](https://help.streak.tech/wiuqecaa5pue8wv2qqwn0e43)
- **Basic mode logic.** Basic mode cannot mix AND and OR. [help](https://help.streak.tech/vbgeqlecn8knaiw9ck8l2qsd)
- **"Crosses above" vs "Higher than".** Crosses above means false on the previous candle and true now. Higher than is the current state only, and counts equal as true. [help](https://help.streak.tech/fr3jgo77twtv520rgzd6i7ni)
- **Instruments per strategy.** Up to **50** per equity strategy; watchlists can be imported. [Equity Strategy](https://help.streak.tech/pfgdsjkva1zd65ika53eyrwk)

### Backtest errors
- **Error messages.** "No Trades Happened", "No Trades Happened Because the Initial Capital Is Low", and "Limit Exceeded" (daily backtest cap, resets the next day; the number is not published). [help](https://help.streak.tech/fszore8axs122z4n7hn7lkfb)

## 2. Metric definitions
- **P&L, W vs L, WS, LS.** P&L is the total over all simulated trades. W vs L is the count of wins against losses. WS is the longest run of consecutive winning trades and LS the longest losing run. [Key Performance Metrics](https://help.streak.tech/sa3dovzi8dbl5rn7m7labku8)
- **Max DD.** The largest peak-to-trough drop in the P&L curve before a new peak. The formula is given as `(Peak − Trough) / Peak`. A strategy with no losing trades has Max DD 0. The doc's examples show both positive and negative values ("0.97 better than 1.53", "−0.12 better than −0.93"), so the sign convention is unclear. [Key Performance Metrics](https://help.streak.tech/sa3dovzi8dbl5rn7m7labku8)
- **Report contents.** P&L curve, max gain/loss, average gain and loss per trade, and max drawdown. [What is Backtest](https://help.streak.tech/eh2wi3mumu9iea27o8qftqat) "Winning/Losing Avg" is not formally defined beyond "average gain and loss per trade".
- **Hypothetical cumulative P&L calendar.** One block per trading day, green or red. Open CNC positions are marked to that day's close against the buy price. [Daily and Monthly Returns](https://help.streak.tech/sr14clgb6sdo2ny2q21b8isb)
- **Hypothetical Daily Summary.** Average return, max profit and max loss per **weekday**. [same](https://help.streak.tech/sr14clgb6sdo2ny2q21b8isb)
- **Hypothetical Monthly Performance.** A month grid, green or red. [same](https://help.streak.tech/sr14clgb6sdo2ny2q21b8isb)
- **Live Backtest cards.** Backtest P&L, Trades, Win vs Loss, Capital Used, Max Gain / Max Loss, and a month-by-month grid. [help](https://help.streak.tech/bcvxbpvzvw7cclsbwsuzgwcb)
- **Results scope.** Results are cumulative across all instruments. A combined download for all instruments is not supported. [staff, Mar 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517?page=6)

## 3. Deployment and alerts
- **Execution tab has two modes.** **Notification-Based Trading (NBT)** and **Backtest**. [Execution Tab](https://help.streak.tech/ugy6q4hbgv7m9wg8029l63fo)
- **NBT flow.** The trigger fires on a closed candle. The user clicks the notification to place the order, and Streak then tracks SL/TP and sends exit alerts. [help](https://help.streak.tech/ytxsts589jmcxbn9431ewgsf), [help](https://help.streak.tech/w6fsx9ngbkv5h9qyy2izbicy)
- **No automation.** Automated trading is not supported pending regulatory approval (Jan 2026). [help](https://help.streak.tech/ilhvfy2r7zivr5ylr3bb6ye0)
- **No paper trading.** Virtual/paper trading is **not supported** on the new platform (Jan 2026). [help](https://help.streak.tech/t5pcwnnhfe8hrh7cdthd2984)
- **[LEGACY/OLD] Earlier deployment modes.** The legacy platform had "Take Live" (notification) and "Paper Trade", in which virtual orders auto-filled. In 2024 the free tier was virtual-only. [Z-Connect v4, 2021](https://zerodha.com/z-connect/streak/introducing-streak-v4), [Z-Connect Jan 2024](https://zerodha.com/z-connect/?p=368853), [TradingQnA Dec 2025](https://tradingqna.com/t/streak-operates-on-different-clocks-it-needs-a-fix/189928)
- **Deployment limit.** Up to **100 concurrent deployments**. Each instrument counts as one, so a 50-stock strategy uses 50. [help](https://help.streak.tech/n20koudvfiv6unhc2qted9uf)
- **Validity.** A deployment tracks entry/TP/SL/exit for **T+29 days**, then stops automatically. [help](https://help.streak.tech/yodsd3j61qq9fbcxllxg0pjc) [LEGACY] This was "30 days max". [TradingQnA](https://tradingqna.com/t/strategy-auto-expired-after-a-month-why/187265)
- **MIS expiry.** MIS deployments expire daily at 3:21 PM. [help](https://help.streak.tech/u35mikvpcigwrd72k6bonzmd)
- **Partial fills.** SL and TP are not monitored for partially filled orders. [help](https://help.streak.tech/lvfdz7y6li6q37v3eepmcnmb), [help](https://help.streak.tech/sjo49n4oc8rwldbqky97svxm)
- **Untracked positions keep showing P&L.** P&L keeps running on the live page if the deployment has no SL/TP or the position was closed on the broker side. [help](https://help.streak.tech/n99aufr4zsfqyqvugaiedefr)
- **Late triggers.** A trigger after 3:30 PM can be placed in the post-market window, 3:40–4:00 PM. [help](https://help.streak.tech/fpbo8ptx64940b5koaovuklc)
- **Latency.** Alerts arrive about **8–14 seconds** after candle close. [help](https://help.streak.tech/qwg1xp15ppb07r4yvbjwjwxi)
- **Notification channels.** Web push (enable it in Profile and allow browser notifications) and the mobile app. [staff Feb 2026](https://tradingqna.com/t/streak-new-beta-version-option-trading/190810/15)
- **No Telegram, email or webhook alerts.** None of these exist. [OLD] [TradingQnA](https://tradingqna.com/t/macd-crossover-and-vwap-alert-to-telegram/70446/4)
- **Internal notification types.** The help-site bundle defines `alert-notification`, `order-notification`, `order-placement`, `order-webhook`, `price-trigger-notification` (SL/TP) and `scanner` (from the help.streak.tech JS bundle; internal, so treat as a hint only).
- **Combined SL/TP.** These are alerts only, so actual loss can exceed the set amount because of fills. [help](https://help.streak.tech/ks8tvyn620qqd1y031an8bv2), [help](https://help.streak.tech/v70f16wex8t3r3l7tij5k2b1)
- **"Live/Offline" badge.** Shows whether the server connection is up. [help](https://help.streak.tech/algc75f86kmz61vy2wiexdc5)
- **Freeze limit.** Orders above the freeze limit are rejected. [help](https://help.streak.tech/n564koqcc0vg8m5mtka3wqvl)
- **Single session.** Only one login session can be active at a time. [help](https://help.streak.tech/ixngthse7mdkgsxrrogr92xy)

## 4. Scanners
- **Universes.** Nifty 50, Nifty 100, Nifty 500 (the broadest), Bank Nifty, custom watchlists, and up to **10 custom baskets of 50 stocks** each. [Create Scanner](https://help.streak.tech/fn2gx7xweb98jt6xek2p89yg), [My Scanners](https://help.streak.tech/dujfreikb8f2b355218xflud), [help](https://help.streak.tech/mihyvr02maicqurqp1pfigzw)
- **Option baskets.** Nifty, Bank Nifty and Sensex, across weekly, next-weekly and monthly expiries. Filters: ATM, ITM or OTM; a strike range; CE, PE or both; Volume > 0. [Create Scanner](https://help.streak.tech/fn2gx7xweb98jt6xek2p89yg)
- **Not supported.** NFO futures and stock-futures baskets (Mar 2026). [staff](https://tradingqna.com/t/meet-the-new-streak-beta/190517?page=6)
- **Frequency.** Alerts fire at every candle close of the scanner timeframe. Custom frequencies are not on the roadmap. [Scanner Alerts](https://help.streak.tech/bki99lk6d0j7xs9lrj1f6sbh), [staff](https://tradingqna.com/t/meet-the-new-streak-beta/190517?page=6)
- **Two modes.** A Manual scan is one-time and checks the current candle. A Live scanner checks continuously. [My Scanners](https://help.streak.tech/dujfreikb8f2b355218xflud)
- **MTF in scanners.** The base timeframe checks the current (live) candle and higher timeframes check their last closed candle. At least one base-TF condition is required; otherwise the error is "At least one condition must use the base timeframe." [Advanced Create: Scanners](https://help.streak.tech/w1fv10mpf5jz6z3jybf8nzuz)
- **Live scanner limit.** **25** at once. [help](https://help.streak.tech/iel46xraxtufutqo3f73eb7k) [OLD 2024: 5](https://zerodha.com/z-connect/?p=368853)
- **Stop alerts.** Pausing is done with "Stop Alerts". [help](https://help.streak.tech/ntrbv5yhn7yxv9tda9msc46d)
- **Condition limits and timeframes.** Basic mode allows 5 conditions and Advanced 10, with Weekly and Monthly only as MTF. The Create Scanner page lists 1, 5, 15 min, 1 h and 1 d. [help](https://help.streak.tech/w1fv10mpf5jz6z3jybf8nzuz), [help](https://help.streak.tech/fn2gx7xweb98jt6xek2p89yg)
- **Catalogue.** 50+ indicators and 50+ prebuilt scanners. [help](https://help.streak.tech/p8p0rdtg1pq6v3oe2ji7uvnt)
- **Orders from results.** Orders can be placed straight from scan results (Market, Limit, SL or SLM), with a technical summary shown first. [help](https://help.streak.tech/tffynisxe4s1fe7h84wfsun8)
- **Scanner-to-strategy linking.** No documented feature. The shared link is that the same watchlists and baskets work in both. [help](https://help.streak.tech/tg71zwoffx4zczh8trps92eu)
- **% change after hours.** The NSE close is the VWAP of 3:00–3:30 PM, so % change can differ after the market closes. [help](https://help.streak.tech/sdaoxgi4p6ix5x3nnyjgy7pj)

## 5. Options strategy builder ("Options Trader")
- **Two modes.** **Underlying-Based** (indicator conditions on the underlying, up to 5 conditions, with an optional second reference instrument) and **Time-Based** (enter at a clock time on chosen weekdays). [Options Strategy](https://help.streak.tech/vuyk3hrzhwveqmtul6il3ge3), [Underlying](https://help.streak.tech/ja5dz8dtnihxkpm4kdfc7oal), [Time](https://help.streak.tech/bwedmi1in5ihbcblhte3l5t4)
- **Legs.** Single or multi-leg, B or S, with live premium, LTP and total premium. Prebuilt strategies are grouped Bullish, Bearish and Neutral (e.g. Bull Call Spread, Call Ratio Back Spread). [Options Strategy](https://help.streak.tech/vuyk3hrzhwveqmtul6il3ge3)
- **Leg order.** Legs execute in the order added. Add the hedge (Buy) first to get the margin benefit. [staff Jan 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517)
- **Strike selection.** Either from the Option Chain or by **Auto Strike Selection**:
  - **ATM-based (ATM ±N).** ATM 1 is one strike above and ATM −1 one strike below, for both CE and PE. [help](https://help.streak.tech/so1lyaw78vacnyw7w58b9d03), [help](https://help.streak.tech/teot30k90e9qlbyagdw4b80s)
  - **Premium-based.** "Premium equal to" picks the nearest premium, which can be above or below. "Premium higher than" and "Premium lower than" pick the nearest on that side. [help](https://help.streak.tech/so1lyaw78vacnyw7w58b9d03)
- **Expiry.** Picked from a dropdown per leg. Sensex weekly, next-weekly and monthly are supported. [help](https://help.streak.tech/so1lyaw78vacnyw7w58b9d03), [help](https://help.streak.tech/vstlg6y4g9uyv9ze774kv1ff)
- **Combined SL and target.** Strategy-level SL/TP triggers an exit notification. [help](https://help.streak.tech/v70f16wex8t3r3l7tij5k2b1)
- **Payoff panel.** Target price slider, Max Profit, Max Loss, breakeven at expiry, Margin Needed and Margin Available. [help](https://help.streak.tech/r2yux4ial3qepa1tlhz688db), [help](https://help.streak.tech/oz1hmyycpeflzqlyyupflgmb)
- **MTF in Options Trader.** Two timeframes are allowed. Evaluation happens only at their LCM, after the higher-TF candle closes. [help](https://help.streak.tech/cmun5e4no1to4l7tzyvr3rds)
- **Discretionary entry.** No native option. The workaround is a "Close higher than 0" entry condition. [staff Jan 2026](https://tradingqna.com/t/streak-new-beta-version-option-trading/190810/15)
- **[LEGACY] Dynamic Contracts.** These were replaced by Auto Strike Selection. [help](https://help.streak.tech/thg2xwteyxqjk07l5uhcibc6)

## 6. Plans and limits
- **Current pricing: free.** Streak is free for Zerodha users and there is no subscription; brokerage still applies. It is Zerodha-only, and no Kite Connect subscription is needed. [help](https://help.streak.tech/s6u88q5uq7laclgpzux1hcy7), [help](https://help.streak.tech/oazqplo4yv7hkr476qf57ioe), [help](https://help.streak.tech/h7nptcdw68d1a8usqfkldom7), [help](https://help.streak.tech/cmv2zq3m8c9jk2mourpsbtqy), [staff Feb 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517). There is no pricing page: www.streak.tech/pricing returns 404.
- **Current limits.**

| Item | Limit | Source |
|---|---|---|
| Concurrent deployments | 100 instrument-deployments | [help](https://help.streak.tech/n20koudvfiv6unhc2qted9uf) |
| Live scanners | 25 | [help](https://help.streak.tech/iel46xraxtufutqo3f73eb7k) |
| Instruments per equity strategy | 50 | [help](https://help.streak.tech/pfgdsjkva1zd65ika53eyrwk) |
| Conditions | 5 Basic / 10 Advanced | [help](https://help.streak.tech/wcyaguzrnqavp82hu2d0xxeu) |
| Watchlist size | 50 | [help](https://help.streak.tech/nsrmmknd3rncu0aa5c6a1euv) |
| Custom baskets | 10 × 50 stocks | [help](https://help.streak.tech/mihyvr02maicqurqp1pfigzw) |
| Backtests/day | capped, number unpublished | [help](https://help.streak.tech/fszore8axs122z4n7hn7lkfb) |
| Scalper chart indicators | 5 per chart | [help](https://help.streak.tech/gvix87i7w1k6y5t1s0i1wa0q) |

- **[OLD] Free "Unlimited" tier (Jan 2024).** Unlimited backtests on equities, 5 live deployments and 5 live scanners, 15–25 virtual deployments, and the NSE 500 universe. [Z-Connect](https://zerodha.com/z-connect/?p=368853)
- **[OLD] Legacy paid plans (2018–2019).** Basic ₹500 + GST/mo (200 backtests/day, 25 deployments), Premium ₹900 (500/day, 50), Ultimate ₹1,400 (1,000/day, 100). A 7-day trial gave 50 backtests and 5 deployments. [Z-Connect 2018 + comments](https://zerodha.com/z-connect/tradezerodha/streak/streak-just-got-a-whole-lot-better). Later Regular/Ultimate prices (around ₹690 and ₹1,400) appear only in third-party reviews. [AlgoTest](https://algotest.in/blog/streak-vs-tradetron.md)

## 7. Other features
- **Assist.** The "Assist" button does guided or autocomplete condition building in Basic Create. No generative AI is documented. [help](https://help.streak.tech/fn2gx7xweb98jt6xek2p89yg), [help](https://help.streak.tech/w1fv10mpf5jz6z3jybf8nzuz), [OLD v4 2021](https://zerodha.com/z-connect/streak/introducing-streak-v4)
- **Discover Strategies.** Filters: Equity Swing, Equity Intraday, Option Buying and Option Selling. Copy a strategy to customise it; positioned as educational only. **Discover Scanners**: "Set Alerts" deploys one live. **Live Backtest** shows prebuilt-strategy results on the home page. [help](https://help.streak.tech/aftu4k6qt3hebqsjy52h7dkn), [help](https://help.streak.tech/sqmfgcyr7hwb3y25fkfh94pa), [help](https://help.streak.tech/vckddbca8ngrrdoa224eg4xv)
- **Sharing.** Share a strategy or scanner by copying its URL; it needs "Public Sharing" enabled. [help](https://help.streak.tech/w3g5lf23zsl3jyetr6aqy20n), [staff](https://tradingqna.com/t/streak-operates-on-different-clocks-it-needs-a-fix/189928)
- **Scalper mode.**
  - **Layout.** Three charts (underlying, ATM CE, and PE through Add Chart). The CE/PE toggle swaps the option chart. The primary chart takes only an index or stock.
  - **Indicators.** Up to 5 per chart, on charts 1 and 2 only.
  - **Timeframes.** 1, 3, 5, 15, 30 min, 1 h and 1 d. Changing chart 1 syncs all charts, which can then be changed individually.
  - **Watchlist.** Enabling the watchlist limits the view to 2 charts.
  - **Trade-from-chart.** Buy shows the best ask and Sell the best bid, with Market as the default order type. Hover a price and click "+" to place a limit or SL order. Drag an order line to modify it; click X to cancel or close.
  - **Margin and positions.** Required margin is shown inside the Buy/Sell buttons. Positions, Open Orders and Order Book have a 3-dot menu (Buy, Sell, Info, Cancel, Modify, Add More, Exit full or partial) and a Total P&L.
  - **Basket orders.** Drag legs to reorder them; placing the hedge first gives the margin benefit.

  Sources: [Scalper](https://help.streak.tech/e07ndyaqohxh2a7qtlenpke6), [Features](https://help.streak.tech/t89spcydm637r34vf6z35j7i), [Charts](https://help.streak.tech/s1cl1nsjzmmm1mz2aqnjgk0f), [Indicators](https://help.streak.tech/gvix87i7w1k6y5t1s0i1wa0q), [TFC](https://help.streak.tech/z28as9woiryua49o6exdxlp8), [Positions](https://help.streak.tech/xiigsosefl0i5vt69aiaj8ga), [Option chain](https://help.streak.tech/vm84uhsul2v7cdjog0iph2zv), [Margin](https://help.streak.tech/la3jjnwssls71a07puxxhflq)
- **Home and Technicals.**
  - **Header.** Index tickers (Nifty 50, Bank Nifty, Sensex), plus Order Book, Positions, Notifications and Profile at top right.
  - **Main chart.** Nifty 50 by default.
  - **Technicals tab.** Bearish/Neutral/Bullish sentiment bar and recent events. Indicators lag about **6–7 minutes**.
  - **Other widgets.** Option Chain (hover a row to show B/S), OI, PCR, Advance/Decline, ATM IV and Straddle, and pivot levels R1–R3 / S1–S3.

  Sources: [Home](https://help.streak.tech/o0q373cpi0c0cfca1jvunp9t), [Charts & insights](https://help.streak.tech/oe1tllunu28ba2fs3exeevfv), [Technicals](https://help.streak.tech/e66dk8ottx6iphzt2n8augxi), [Pivots](https://help.streak.tech/nirltk6qz1lr6wls66jtyv44)
- **Chart stack.** TradingView charts for trading and scalping; ChartIQ charts next to strategy conditions. New indicators can be added only if ChartIQ supports them. [staff Jan 2026](https://tradingqna.com/t/meet-the-new-streak-beta/190517)
- **Mobile.** Android (`com.open.streak`) and iOS (id6476100104). Scanner alerts reach both web and app. The team is focused on the app (Apr–May 2026). No web-vs-app feature matrix is published. [help](https://help.streak.tech/hgf9tsysfms8tvlpicbaub7l), [help](https://help.streak.tech/tvn5t2x0syf1f9zjeaob3xxz)
- **Platform and support.** Chrome is recommended. No public API. Support is by email only, at support@streak.tech. [help](https://help.streak.tech/dfzwhqnyk5kblnrzwy0292k1), [help](https://help.streak.tech/viyffgenvqumob78kx2vyn94), [help](https://help.streak.tech/ddhcyexno68pzlhybneib3h0)
- **Keyboard shortcuts.** No public documentation found. The only hint is a toast region labelled "Notifications alt+T" in the streak.tech page markup.

---

## Unverified / conflicting

1. **SL/TP evaluation.** The Backtest Assumptions page says both "SL/TP evaluated on the close price of every candle" and "an O→L→H→C path for TSL, with SL assumed first when both hit in the same candle". Most likely, TSL and same-candle checks use high/low and the rest is close-based, but this is not confirmed.
2. **Square-off times disagree.** The generic "3:15 PM candle open" (Backtest Assumptions, Mar 2026; EN/EX/SO FAQ, Dec 2025) conflicts with the per-instrument 3:10 and 3:25 times (FAQs dated Sep 2026). Treat the Sep 2026 per-instrument rule as current.
3. **Max DD sign and formula.** `(Peak − Trough)/Peak` is a ratio, yet the doc's examples include negative values. The units (₹ or %) are unclear.
4. **Backtests/day cap.** The cap exists ("Limit Exceeded") but no number is published. A third-party review claims "1000+".
5. **Paper trading.** The streak.tech meta description still says "virtually deploy strategies", while the help (Jan 2026) says virtual trading is not supported. Treat it as unsupported.
6. **Live MIS cut-off.** One FAQ says MIS stops triggering after 3:20 PM and another says MIS deployments expire at 3:21 PM. These are consistent but approximate.
7. **Scanner timeframe list.** The Create Scanner doc lists 1/5/15 min, 1 h and 1 d, but MTF docs reference 3, 10 and 30 min base timeframes too. The full scanner list is unconfirmed.
8. **Not documented at all.** Slippage setting, stamp duty in charges, Max Allocation on the new platform, Telegram/email alerts on the new platform, keyboard shortcuts, AI features, and mobile-vs-web differences. All need UI observation.
9. **Old-platform pricing.** This is mostly from 2018–2019 Z-Connect comments and third-party reviews. It is irrelevant now that Streak is free, but kept for history.
