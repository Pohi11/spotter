// Package postgres is the registry's ports.Repository on PostgreSQL (pgx v5,
// hand-written SQL, decision record 0005).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pohi11/spotter/services/registry/internal/domain"
	"github.com/Pohi11/spotter/services/registry/internal/ports"
)

// Constraint names the adapter turns into domain errors.
const (
	uniqueViolation    = "23505"
	modelNameKey       = "models_name_key"
	oneProductionIndex = "model_versions_one_production_per_model"
	versionRevisionKey = "stage_changes_version_revision_key"
)

// Repository implements ports.Repository.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a repository using pool. The schema must already be migrated.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ ports.Repository = (*Repository)(nil)

func (r *Repository) CreateModel(ctx context.Context, m domain.Model) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO models (id, name, description, created_at) VALUES ($1, $2, $3, $4)`,
		m.ID, m.Name, m.Description, m.CreatedAt)
	if isUnique(err, modelNameKey) {
		return domain.ErrAlreadyExists
	}
	return err
}

func (r *Repository) ModelByName(ctx context.Context, name string) (domain.Model, error) {
	return r.model(ctx, `SELECT id, name, description, created_at FROM models WHERE name = $1`, name)
}

func (r *Repository) ModelByID(ctx context.Context, id uuid.UUID) (domain.Model, error) {
	return r.model(ctx, `SELECT id, name, description, created_at FROM models WHERE id = $1`, id)
}

func (r *Repository) model(ctx context.Context, sql string, arg any) (domain.Model, error) {
	var m domain.Model
	err := r.pool.QueryRow(ctx, sql, arg).Scan(&m.ID, &m.Name, &m.Description, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Model{}, domain.ErrNotFound
	}
	m.CreatedAt = m.CreatedAt.UTC()
	return m, err
}

func (r *Repository) AddVersion(ctx context.Context, v domain.ModelVersion, events func(domain.ModelVersion) ([]ports.OutboxRecord, error)) (domain.ModelVersion, error) {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// Lock the model row: concurrent registrations for this model wait here, so
		// max(version)+1 is never read by two transactions at once (design D4).
		var locked uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM models WHERE id = $1 FOR UPDATE`, v.ModelID).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			`SELECT coalesce(max(version), 0) + 1 FROM model_versions WHERE model_id = $1`, v.ModelID,
		).Scan(&v.Number); err != nil {
			return err
		}
		metrics, err := json.Marshal(v.Metrics)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO model_versions
				(id, model_id, version, artifact_uri, artifact_sha256, framework, metrics, stage, revision, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			v.ID, v.ModelID, v.Number, v.ArtifactURI, v.ArtifactSHA256, string(v.Framework), metrics,
			string(v.Stage), v.Revision, v.CreatedAt, v.UpdatedAt); err != nil {
			return err
		}
		records, err := events(v)
		if err != nil {
			return err
		}
		return insertOutbox(ctx, tx, records)
	})
	if err != nil {
		return domain.ModelVersion{}, err
	}
	return v, nil
}

const versionColumns = `id, model_id, version, artifact_uri, artifact_sha256, framework, metrics, stage, revision, created_at, updated_at`

func (r *Repository) Version(ctx context.Context, id uuid.UUID) (domain.ModelVersion, error) {
	v, err := scanVersion(r.pool.QueryRow(ctx, `SELECT `+versionColumns+` FROM model_versions WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ModelVersion{}, domain.ErrNotFound
	}
	return v, err
}

func (r *Repository) Versions(ctx context.Context, modelID uuid.UUID) ([]domain.ModelVersion, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+versionColumns+` FROM model_versions WHERE model_id = $1 ORDER BY version DESC`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ModelVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) SaveTransition(ctx context.Context, v domain.ModelVersion, expectedRevision int64, change domain.StageChange, records []ports.OutboxRecord) error {
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		// The revision condition is the real concurrency check (design D2). Under READ
		// COMMITTED a racing UPDATE waits for the first to commit, re-reads the row, sees
		// the new revision, and matches nothing.
		tag, err := tx.Exec(ctx,
			`UPDATE model_versions SET stage = $1, revision = $2, updated_at = $3 WHERE id = $4 AND revision = $5`,
			string(v.Stage), v.Revision, v.UpdatedAt, v.ID, expectedRevision)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM model_versions WHERE id = $1)`, v.ID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return domain.ErrNotFound
			}
			return domain.ErrConflict
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stage_changes (version_id, from_stage, to_stage, reason, revision, changed_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			change.VersionID, string(change.From), string(change.To), change.Reason, change.Revision, change.At); err != nil {
			return err
		}
		return insertOutbox(ctx, tx, records)
	})
	switch {
	case isUnique(err, oneProductionIndex):
		return domain.ErrProductionOccupied
	case isUnique(err, versionRevisionKey):
		return domain.ErrConflict
	}
	return err
}

func (r *Repository) History(ctx context.Context, versionID uuid.UUID) ([]domain.StageChange, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT version_id, from_stage, to_stage, reason, revision, changed_at
		FROM stage_changes WHERE version_id = $1 ORDER BY revision`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.StageChange
	for rows.Next() {
		var c domain.StageChange
		var from, to string
		if err := rows.Scan(&c.VersionID, &from, &to, &c.Reason, &c.Revision, &c.At); err != nil {
			return nil, err
		}
		c.From, c.To, c.At = domain.Stage(from), domain.Stage(to), c.At.UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) PendingOutbox(ctx context.Context, limit int) ([]ports.OutboxRecord, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_type, aggregate_id, payload, occurred_at
		FROM outbox WHERE published_at IS NULL ORDER BY occurred_at, id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ports.OutboxRecord
	for rows.Next() {
		var rec ports.OutboxRecord
		if err := rows.Scan(&rec.ID, &rec.Type, &rec.AggregateID, &rec.Payload, &rec.OccurredAt); err != nil {
			return nil, err
		}
		rec.OccurredAt = rec.OccurredAt.UTC()
		out = append(out, rec)
	}
	return out, rows.Err()
}

func insertOutbox(ctx context.Context, tx pgx.Tx, records []ports.OutboxRecord) error {
	for _, rec := range records {
		if _, err := tx.Exec(ctx, `
			INSERT INTO outbox (id, event_type, aggregate_id, payload, occurred_at)
			VALUES ($1, $2, $3, $4, $5)`,
			rec.ID, rec.Type, rec.AggregateID, rec.Payload, rec.OccurredAt); err != nil {
			return fmt.Errorf("insert outbox record %s: %w", rec.ID, err)
		}
	}
	return nil
}

func scanVersion(row pgx.Row) (domain.ModelVersion, error) {
	var v domain.ModelVersion
	var framework, stage string
	var metrics []byte
	if err := row.Scan(&v.ID, &v.ModelID, &v.Number, &v.ArtifactURI, &v.ArtifactSHA256, &framework, &metrics,
		&stage, &v.Revision, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return domain.ModelVersion{}, err
	}
	v.Framework, v.Stage = domain.Framework(framework), domain.Stage(stage)
	v.CreatedAt, v.UpdatedAt = v.CreatedAt.UTC(), v.UpdatedAt.UTC()
	v.Metrics = map[string]float64{}
	if err := json.Unmarshal(metrics, &v.Metrics); err != nil {
		return domain.ModelVersion{}, fmt.Errorf("decode metrics: %w", err)
	}
	return v, nil
}

func isUnique(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == constraint
}
