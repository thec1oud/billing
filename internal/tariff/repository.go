package tariff

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type Repository interface {
	Save(ctx context.Context, t Tariff) (Tariff, error)
	GetByCodeAndVersion(ctx context.Context, code string, version int) (Tariff, error)
	LatestVersion(ctx context.Context, code string) (int, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Save(ctx context.Context, t Tariff) (Tariff, error) {
	query := `
		INSERT INTO tariffs (
			tariff_code, version, name, tariff_type_code, amount, billing_interval_code, is_active, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, COALESCE($7, TRUE), COALESCE($8, '{}'::jsonb)
		)
		RETURNING tariff_id, created_at;
	`

	metadataBytes := t.Metadata
	if len(t.Tiers) > 0 {
		// Embed tier structures within metadata JSONB if not using a separate child table
		tierMap := map[string]interface{}{
			"tiers": t.Tiers,
		}
		if len(metadataBytes) > 0 {
			var existing map[string]interface{}
			_ = json.Unmarshal(metadataBytes, &existing)
			existing["tiers"] = t.Tiers
			metadataBytes, _ = json.Marshal(existing)
		} else {
			metadataBytes, _ = json.Marshal(tierMap)
		}
	}

	var tariffID int64
	var createdAt time.Time

	err := r.db.QueryRowContext(
		ctx,
		query,
		t.TariffCode,
		t.Version,
		t.Name,
		t.TariffTypeCode,
		t.Amount.AmountMinor,
		t.BillingIntervalCode,
		t.IsActive,
		metadataBytes,
	).Scan(&tariffID, &createdAt)
	if err != nil {
		return Tariff{}, fmt.Errorf("failed to insert tariff: %w", err)
	}

	t.ID = tariffID
	t.CreatedAt = createdAt
	t.Metadata = metadataBytes
	return t, nil
}

func (r *postgresRepository) GetByCodeAndVersion(ctx context.Context, code string, version int) (Tariff, error) {
	query := `
		SELECT 
			tariff_id, tariff_code, version, name, tariff_type_code, amount, billing_interval_code, is_active, metadata, created_at
		FROM tariffs
		WHERE tariff_code = $1 AND version = $2;
	`

	var t Tariff
	var rawAmount int64
	var rawType, rawInterval string

	err := r.db.QueryRowContext(ctx, query, code, version).Scan(
		&t.ID,
		&t.TariffCode,
		&t.Version,
		&t.Name,
		&rawType,
		&rawAmount,
		&rawInterval,
		&t.IsActive,
		&t.Metadata,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Tariff{}, fmt.Errorf("tariff %s v%d not found", code, version)
		}
		return Tariff{}, fmt.Errorf("failed to query tariff: %w", err)
	}

	t.TariffTypeCode = TariffTypeCode(rawType)
	t.BillingIntervalCode = BillingInterval(rawInterval)
	
	// Currency defaults to system default or metadata field
	m, _ := money.New(rawAmount, "USD")
	t.Amount = m

	// Unmarshal embedded tiers if available
	if len(t.Metadata) > 0 {
		var meta struct {
			Tiers []Tier `json:"tiers"`
		}
		if err := json.Unmarshal(t.Metadata, &meta); err == nil && len(meta.Tiers) > 0 {
			t.Tiers = meta.Tiers
		}
	}

	return t, nil
}

func (r *postgresRepository) LatestVersion(ctx context.Context, code string) (int, error) {
	query := `
		SELECT COALESCE(MAX(version), 0)
		FROM tariffs
		WHERE tariff_code = $1;
	`

	var latest int
	err := r.db.QueryRowContext(ctx, query, code).Scan(&latest)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch latest tariff version: %w", err)
	}

	return latest, nil
}