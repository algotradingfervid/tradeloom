-- Tradeloom app.db schema v1 (ARCHITECTURE §4.2, P0 tables). Executed inside one transaction by store.Migrator.
-- No PRAGMA, ATTACH or transaction statements are allowed in migration files.

-- settings
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id TEXT, logged_in_at INTEGER, expires_at INTEGER,
  risk_ack_at INTEGER, data_plan TEXT, ended_at INTEGER, end_reason TEXT);
CREATE TABLE shortcuts (action_id TEXT PRIMARY KEY, binding TEXT NOT NULL, scope TEXT NOT NULL);
CREATE TABLE window_state (k TEXT PRIMARY KEY, v TEXT NOT NULL);

-- instruments (replaced wholesale per import, inside one transaction)
CREATE TABLE instruments (token INTEGER PRIMARY KEY, exchange_token INTEGER, tradingsymbol TEXT NOT NULL,
  name TEXT, exchange TEXT NOT NULL, segment TEXT NOT NULL, instrument_type TEXT NOT NULL,
  expiry INTEGER, tick_size_paise INTEGER NOT NULL, lot_size INTEGER NOT NULL, display_name TEXT NOT NULL,
  underlying TEXT, is_index INTEGER NOT NULL, is_weekly INTEGER, imported_at INTEGER NOT NULL);
CREATE INDEX ix_instr_symbol ON instruments(exchange, tradingsymbol);
CREATE INDEX ix_instr_underlying ON instruments(underlying, expiry);
CREATE TABLE instrument_imports (id INTEGER PRIMARY KEY, downloaded_at INTEGER, row_count INTEGER, kept_count INTEGER, source TEXT);
CREATE VIRTUAL TABLE instrument_fts USING fts5(tradingsymbol, name, display_name, content='instruments', content_rowid='token', tokenize='unicode61');

-- watchlists
CREATE TABLE watchlists (id TEXT PRIMARY KEY, name TEXT NOT NULL, position INTEGER NOT NULL, created_at, updated_at, deleted_at);
CREATE TABLE watchlist_items (watchlist_id TEXT REFERENCES watchlists(id), token INTEGER, position INTEGER, PRIMARY KEY(watchlist_id, token));

-- orders (local cache + journal)
CREATE TABLE order_journal (id INTEGER PRIMARY KEY, at INTEGER NOT NULL, tag TEXT NOT NULL, kind TEXT NOT NULL,
  source TEXT NOT NULL, params TEXT, kite_order_id TEXT, status TEXT, message TEXT, stage TEXT);
CREATE INDEX ix_journal_tag ON order_journal(tag, at);
CREATE INDEX ix_journal_at ON order_journal(at);
CREATE TABLE orders_cache (kite_order_id TEXT PRIMARY KEY, tag TEXT, body TEXT NOT NULL, status TEXT, updated_at);
CREATE TABLE positions_cache (k TEXT PRIMARY KEY, body TEXT NOT NULL, updated_at);
CREATE TABLE order_counters (day INTEGER PRIMARY KEY, orders INTEGER NOT NULL, mods TEXT, http429 INTEGER NOT NULL DEFAULT 0);

-- notifications inbox (empty until P3)
CREATE TABLE notifications (id TEXT PRIMARY KEY, at INTEGER NOT NULL, type TEXT NOT NULL, category TEXT,
  deployment_id TEXT, di_id TEXT, scanner_id TEXT, tokens TEXT, title TEXT, body TEXT NOT NULL,
  status TEXT, expires_at INTEGER, read_at INTEGER, resolved_at INTEGER, resolved_by TEXT);
CREATE INDEX ix_notif_at ON notifications(at);
CREATE INDEX ix_notif_status ON notifications(status, expires_at);

-- calendar edits
CREATE TABLE calendar_overrides (exchange TEXT, date INTEGER, type TEXT, phases TEXT, source TEXT, PRIMARY KEY(exchange, date));
