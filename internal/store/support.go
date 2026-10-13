package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"tradeloom/internal/logging"
)

// JournalTailRows is how many order-journal rows a support bundle carries (FR-SHELL-018).
const JournalTailRows = 200

type journalRow struct {
	ID          int64   `json:"id"`
	At          int64   `json:"at"`
	Tag         string  `json:"tag"`
	Kind        string  `json:"kind"`
	Source      string  `json:"source"`
	Params      *string `json:"params,omitempty"`
	KiteOrderID *string `json:"kite_order_id,omitempty"`
	Status      *string `json:"status,omitempty"`
	Message     *string `json:"message,omitempty"`
	Stage       *string `json:"stage,omitempty"`
}

// credentialKeyPrefix names the Kite credential settings (kite.api_key, kite.redirect_port). They are not
// secret-like by name, but the API key identifies the account: service.Settings hides them from the UI
// (hiddenKey) and the support bundle must not ship them either. The prefix is spelled here because store
// is a leaf that cannot import internal/kite.
const credentialKeyPrefix = "kite."

// SupportFiles returns the app.db part of a support bundle: settings.json (the settings table never
// holds secrets; the Kite credential keys are left out), versions.json, order_journal.json (newest 200
// rows) and deployment_logs.json (empty until the P4 deployment_events table exists).
func (db *DB) SupportFiles(ctx context.Context) ([]logging.BundleFile, error) {
	set, err := db.SettingsRepo().Load(ctx)
	if err != nil {
		return nil, err
	}
	setJSON := map[string]json.RawMessage{}
	for k, v := range set {
		if logging.IsSecretLikeKey(k) || strings.HasPrefix(k, credentialKeyPrefix) {
			continue // defence in depth: settings never hold secrets, but a stray row must not ship
		}
		if json.Valid([]byte(v)) {
			setJSON[k] = json.RawMessage(v)
		}
	}
	rows, err := db.R.QueryContext(ctx,
		`SELECT id, at, tag, kind, source, params, kite_order_id, status, message, stage
		 FROM order_journal ORDER BY at DESC, id DESC LIMIT ?`, JournalTailRows)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	journal := []journalRow{}
	for rows.Next() {
		var r journalRow
		var p, k, s, m, st sql.NullString
		if err := rows.Scan(&r.ID, &r.At, &r.Tag, &r.Kind, &r.Source, &p, &k, &s, &m, &st); err != nil {
			return nil, err
		}
		r.Params, r.KiteOrderID, r.Status, r.Message, r.Stage = ns(p), ns(k), ns(s), ns(m), ns(st)
		journal = append(journal, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	versions := map[string]any{"schema": db.schema, "app_version_last": readAppVersion(ctx, db.R)}
	files := []logging.BundleFile{}
	for _, f := range []struct {
		name string
		v    any
	}{{"settings.json", setJSON}, {"versions.json", versions}, {"order_journal.json", journal}, {"deployment_logs.json", []any{}}} {
		b, err := json.MarshalIndent(f.v, "", "  ")
		if err != nil {
			return nil, err
		}
		files = append(files, logging.BundleFile{Name: f.name, Data: b})
	}
	return files, nil
}

func ns(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
