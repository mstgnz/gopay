package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Payment channels: how a payment row was started.
const (
	ChannelCard        = "card"
	ChannelCard3D      = "card_3d"
	ChannelSavedCard   = "saved_card"
	ChannelSavedCard3D = "saved_card_3d"
)

// Payment event kinds recorded in payment_events.
const (
	EventStart    = "start"
	EventCallback = "callback"
	EventStatus   = "status"
	EventCancel   = "cancel"
	EventRefund   = "refund"
)

// Statuses only GoPay sets: a row before the provider answered, and an attempt that ended in a Go error.
const (
	paymentInitiated PaymentStatus = "initiated"
	paymentError     PaymentStatus = "error"
)

// ErrPaymentRowNotFound means no payments row matches; payments started before the table existed have none.
var ErrPaymentRowNotFound = errors.New("payment row not found")

// NewPaymentRecord is a payment attempt as GoPay knows it before calling the provider.
type NewPaymentRecord struct {
	TenantID          int
	Provider          string
	Environment       string
	Channel           string
	ConsumerReference string
	Amount            float64
	Currency          string
	Installment       int
	StartLogID        int64
}

// PaymentEvent is one outcome for a payments row. Empty fields leave the row as it is.
type PaymentEvent struct {
	Kind string
	// Status is the outcome of this event (a refund event's own result), not necessarily the payment's.
	Status    PaymentStatus
	ErrorCode string
	Amount    *float64
	LogID     int64
	// PaymentStatus is the payment's new status; empty keeps the current one.
	PaymentStatus     PaymentStatus
	ProviderPaymentID string
	// OnlyOnChange writes nothing unless PaymentStatus differs from the stored status; status polls
	// repeat the same answer many times per payment.
	OnlyOnChange bool
}

// PaymentKey finds a payment by the id its consumer holds.
type PaymentKey struct {
	TenantID          int
	Provider          string
	Environment       string
	ProviderPaymentID string
}

// PaymentStore persists payments and their events. The services own it; providers never see it.
type PaymentStore interface {
	InsertPayment(ctx context.Context, rec NewPaymentRecord) (int64, error)
	RecordEvent(ctx context.Context, paymentRowID int64, ev PaymentEvent) error
	// FindPayment returns the newest row for the key: a consumer id is not unique per tenant.
	FindPayment(ctx context.Context, key PaymentKey) (int64, error)
	FindPaymentByStartLog(ctx context.Context, tenantID int, providerName string, startLogID int64) (int64, error)
}

// SQLPaymentStore is the PostgreSQL PaymentStore.
type SQLPaymentStore struct {
	db *sql.DB
}

// NewPaymentStore creates a payment store over the shared connection.
func NewPaymentStore(db *sql.DB) *SQLPaymentStore {
	return &SQLPaymentStore{db: db}
}

// InsertPayment stores a new attempt as initiated. An unknown or inactive provider fails the insert.
func (s *SQLPaymentStore) InsertPayment(ctx context.Context, rec NewPaymentRecord) (int64, error) {
	if rec.TenantID <= 0 {
		return 0, fmt.Errorf("invalid tenant ID: %d", rec.TenantID)
	}
	query := `
		INSERT INTO payments (
			tenant_id, provider_id, environment, channel, consumer_reference,
			amount, currency, installment, status, start_log_id
		) VALUES (
			$1, (SELECT id FROM providers WHERE name = $2 AND active ORDER BY id LIMIT 1), $3, $4, $5,
			$6, $7, $8, $9, $10
		)
		RETURNING id`

	var id int64
	err := s.db.QueryRowContext(ctx, query,
		rec.TenantID, rec.Provider, rec.Environment, rec.Channel, nullString(rec.ConsumerReference),
		rec.Amount, rec.Currency, rec.Installment, string(paymentInitiated), nullInt64(rec.StartLogID),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to insert payment: %w", err)
	}
	return id, nil
}

// RecordEvent updates the row and appends the event in one statement, so neither lands without the
// other. With OnlyOnChange an unchanged status writes nothing and is not an error.
func (s *SQLPaymentStore) RecordEvent(ctx context.Context, paymentRowID int64, ev PaymentEvent) error {
	query := `
		WITH upd AS (
			UPDATE payments SET
				status = COALESCE($2::text, status),
				error_code = CASE WHEN $2::text IS NULL THEN error_code ELSE $3::text END,
				provider_payment_id = COALESCE($4::text, provider_payment_id),
				updated_at = now()
			WHERE id = $1 AND (NOT $5::boolean OR status IS DISTINCT FROM $2::text)
			RETURNING id
		)
		INSERT INTO payment_events (payment_id, kind, status, error_code, amount, log_id)
		SELECT id, $6, $7::text, $8::text, $9::numeric, $10::bigint FROM upd`

	// Without a new status there is nothing to compare, so the event is always written.
	onlyOnChange := ev.OnlyOnChange && ev.PaymentStatus != ""
	res, err := s.db.ExecContext(ctx, query,
		paymentRowID, nullStatus(ev.PaymentStatus), nullString(ev.ErrorCode), nullString(ev.ProviderPaymentID),
		onlyOnChange,
		ev.Kind, nullStatus(ev.Status), nullString(ev.ErrorCode), nullFloat(ev.Amount), nullInt64(ev.LogID),
	)
	if err != nil {
		return fmt.Errorf("failed to record payment event: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read payment event result: %w", err)
	}
	if affected == 0 && !onlyOnChange {
		return fmt.Errorf("payment %d: %w", paymentRowID, ErrPaymentRowNotFound)
	}
	return nil
}

// FindPayment returns the newest row for the key.
func (s *SQLPaymentStore) FindPayment(ctx context.Context, key PaymentKey) (int64, error) {
	query := `
		SELECT p.id FROM payments p
		JOIN providers pr ON pr.id = p.provider_id
		WHERE p.tenant_id = $1 AND pr.name = $2 AND p.environment = $3 AND p.provider_payment_id = $4
		ORDER BY p.id DESC
		LIMIT 1`
	return s.scanID(s.db.QueryRowContext(ctx, query, key.TenantID, key.Provider, key.Environment, key.ProviderPaymentID))
}

// FindPaymentByStartLog returns the row a 3D callback belongs to: its state carries the start's log id.
func (s *SQLPaymentStore) FindPaymentByStartLog(ctx context.Context, tenantID int, providerName string, startLogID int64) (int64, error) {
	query := `
		SELECT p.id FROM payments p
		JOIN providers pr ON pr.id = p.provider_id
		WHERE pr.name = $1 AND p.start_log_id = $2 AND p.tenant_id = $3`
	return s.scanID(s.db.QueryRowContext(ctx, query, providerName, startLogID, tenantID))
}

func (s *SQLPaymentStore) scanID(row *sql.Row) (int64, error) {
	var id int64
	if err := row.Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrPaymentRowNotFound
		}
		return 0, fmt.Errorf("failed to find payment: %w", err)
	}
	return id, nil
}

func nullStatus(s PaymentStatus) any {
	if s == "" {
		return nil
	}
	return string(s)
}

func nullInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
