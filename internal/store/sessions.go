// internal/store/sessions.go
package store

import (
	"context"
	"database/sql"
	"errors"
)

// SessionRow mirrors the sessions table; zero values stand for NULL.
type SessionRow struct {
	ID           string
	UserID       string
	LoggedInAtMs int64
	ExpiresAtMs  int64
	RiskAckAtMs  int64
	DataPlan     string
	EndedAtMs    int64
	EndReason    string
}

type SessionsRepo struct{ db *DB }

func NewSessionsRepo(db *DB) *SessionsRepo { return &SessionsRepo{db: db} }

func (r *SessionsRepo) Insert(ctx context.Context, row SessionRow) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, logged_in_at, expires_at, risk_ack_at, data_plan, ended_at, end_reason)
		 VALUES(?, ?, ?, ?, NULL, ?, NULL, NULL)`,
		row.ID, row.UserID, row.LoggedInAtMs, row.ExpiresAtMs, row.DataPlan)
	return err
}

func (r *SessionsRepo) SetDataPlan(ctx context.Context, id, plan string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE sessions SET data_plan = ? WHERE id = ?`, plan, id)
	return err
}

func (r *SessionsRepo) AckRisk(ctx context.Context, id string, atMs int64) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE sessions SET risk_ack_at = ? WHERE id = ? AND risk_ack_at IS NULL`, atMs, id)
	return err
}

func (r *SessionsRepo) End(ctx context.Context, id string, atMs int64, reason string) error {
	_, err := r.db.W.ExecContext(ctx,
		`UPDATE sessions SET ended_at = ?, end_reason = ? WHERE id = ? AND ended_at IS NULL`, atMs, reason, id)
	return err
}

func (r *SessionsRepo) Get(ctx context.Context, id string) (SessionRow, error) {
	var (
		row                           SessionRow
		user, plan, reason            sql.NullString
		loggedIn, expires, ack, ended sql.NullInt64
	)
	err := r.db.R.QueryRowContext(ctx,
		`SELECT id, user_id, logged_in_at, expires_at, risk_ack_at, data_plan, ended_at, end_reason FROM sessions WHERE id = ?`, id).
		Scan(&row.ID, &user, &loggedIn, &expires, &ack, &plan, &ended, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRow{}, ErrNoRow
	}
	if err != nil {
		return SessionRow{}, err
	}
	row.UserID, row.DataPlan, row.EndReason = user.String, plan.String, reason.String
	row.LoggedInAtMs, row.ExpiresAtMs, row.RiskAckAtMs, row.EndedAtMs = loggedIn.Int64, expires.Int64, ack.Int64, ended.Int64
	return row, nil
}
