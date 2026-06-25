package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrSavedCardNotFound is returned when a saved card row does not exist for the given tenant scope.
var ErrSavedCardNotFound = errors.New("saved card not found")

// SavedCard is a GoPay-owned record mapping a tenant + customer MSISDN to a provider wallet card.
// It never stores full PAN or CVV: only the provider card id and masked metadata.
type SavedCard struct {
	ID             int        `json:"id"`
	TenantID       int        `json:"tenantId"`
	ProviderID     int        `json:"providerId"`
	Environment    string     `json:"environment"`
	MSISDN         string     `json:"msisdn"`
	ProviderCardID string     `json:"providerCardId"`
	MaskedCardNo   string     `json:"maskedCardNo,omitempty"`
	CardBrand      string     `json:"cardBrand,omitempty"`
	CardType       string     `json:"cardType,omitempty"`
	Alias          string     `json:"alias,omitempty"`
	IsActive       bool       `json:"isActive"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty"`
}

// SavedCardRepository persists saved cards. Every query is scoped by tenant_id so a tenant can
// never read or charge another tenant's saved card (IDOR/BOLA guard).
type SavedCardRepository struct {
	db *sql.DB
}

// NewSavedCardRepository creates a repository over the shared *sql.DB connection.
func NewSavedCardRepository(db *sql.DB) *SavedCardRepository {
	return &SavedCardRepository{db: db}
}

// Create inserts a saved card, or reactivates/updates the masked metadata if the same
// (tenant, provider, environment, msisdn, provider_card_id) already exists (idempotent via the
// partial unique index). Returns the row id.
func (r *SavedCardRepository) Create(ctx context.Context, card *SavedCard) (int, error) {
	if card.TenantID <= 0 {
		return 0, errors.New("tenant id is required")
	}
	if card.ProviderCardID == "" || card.MSISDN == "" {
		return 0, errors.New("msisdn and provider card id are required")
	}

	query := `
		INSERT INTO saved_cards (
			tenant_id, provider_id, environment, msisdn, provider_card_id,
			masked_card_no, card_brand, card_type, alias, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true)
		ON CONFLICT (tenant_id, provider_id, environment, msisdn, provider_card_id) WHERE is_active
		DO UPDATE SET
			masked_card_no = EXCLUDED.masked_card_no,
			card_brand     = EXCLUDED.card_brand,
			card_type      = EXCLUDED.card_type,
			alias          = EXCLUDED.alias,
			updated_at     = now()
		RETURNING id`

	var id int
	err := r.db.QueryRowContext(ctx, query,
		card.TenantID, card.ProviderID, card.Environment, card.MSISDN, card.ProviderCardID,
		nullString(card.MaskedCardNo), nullString(card.CardBrand), nullString(card.CardType), nullString(card.Alias),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create saved card: %w", err)
	}

	return id, nil
}

// GetByID returns an active saved card by row id, scoped to the tenant.
func (r *SavedCardRepository) GetByID(ctx context.Context, tenantID, id int) (*SavedCard, error) {
	query := `
		SELECT id, tenant_id, provider_id, environment, msisdn, provider_card_id,
		       COALESCE(masked_card_no, ''), COALESCE(card_brand, ''), COALESCE(card_type, ''),
		       COALESCE(alias, ''), is_active, created_at, updated_at
		FROM saved_cards
		WHERE id = $1 AND tenant_id = $2 AND is_active`

	return r.scanOne(r.db.QueryRowContext(ctx, query, id, tenantID))
}

// ListByMsisdn returns the active saved cards for a tenant + provider + environment + msisdn.
func (r *SavedCardRepository) ListByMsisdn(ctx context.Context, tenantID, providerID int, environment, msisdn string) ([]SavedCard, error) {
	query := `
		SELECT id, tenant_id, provider_id, environment, msisdn, provider_card_id,
		       COALESCE(masked_card_no, ''), COALESCE(card_brand, ''), COALESCE(card_type, ''),
		       COALESCE(alias, ''), is_active, created_at, updated_at
		FROM saved_cards
		WHERE tenant_id = $1 AND provider_id = $2 AND environment = $3 AND msisdn = $4 AND is_active
		ORDER BY id DESC`

	rows, err := r.db.QueryContext(ctx, query, tenantID, providerID, environment, msisdn)
	if err != nil {
		return nil, fmt.Errorf("failed to list saved cards: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cards []SavedCard
	for rows.Next() {
		card, err := r.scanOne(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, *card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate saved cards: %w", err)
	}

	return cards, nil
}

// SoftDeleteByID deactivates a saved card by row id, scoped to the tenant.
func (r *SavedCardRepository) SoftDeleteByID(ctx context.Context, tenantID, id int) error {
	query := `UPDATE saved_cards SET is_active = false, updated_at = now() WHERE id = $1 AND tenant_id = $2 AND is_active`
	res, err := r.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete saved card: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read delete result: %w", err)
	}
	if affected == 0 {
		return ErrSavedCardNotFound
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func (r *SavedCardRepository) scanOne(row rowScanner) (*SavedCard, error) {
	var c SavedCard
	var updatedAt sql.NullTime
	err := row.Scan(
		&c.ID, &c.TenantID, &c.ProviderID, &c.Environment, &c.MSISDN, &c.ProviderCardID,
		&c.MaskedCardNo, &c.CardBrand, &c.CardType, &c.Alias, &c.IsActive, &c.CreatedAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSavedCardNotFound
		}
		return nil, fmt.Errorf("failed to scan saved card: %w", err)
	}
	if updatedAt.Valid {
		c.UpdatedAt = &updatedAt.Time
	}
	return &c, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
