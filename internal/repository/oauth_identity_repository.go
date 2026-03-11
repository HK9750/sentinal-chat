package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	sentinal_errors "sentinal-chat/pkg/errors"
)

func nullableOAuthEmail(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

type PostgresOAuthIdentityRepository struct {
	db DBTX
}

func NewOAuthIdentityRepository(db DBTX) OAuthIdentityRepository {
	return &PostgresOAuthIdentityRepository{db: db}
}

func (r *PostgresOAuthIdentityRepository) Create(ctx context.Context, identity *OAuthIdentity) error {
	if identity == nil {
		return sentinal_errors.ErrInvalidInput
	}

	now := time.Now().UTC()
	if identity.ID == uuid.Nil {
		identity.ID = uuid.New()
	}
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = now
	}
	if identity.UpdatedAt.IsZero() {
		identity.UpdatedAt = now
	}

	err := r.db.QueryRowContext(ctx, `
        INSERT INTO oauth_identities (
            id, user_id, provider, provider_user_id, provider_email, email_verified, created_at, updated_at
        )
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
        RETURNING created_at, updated_at
    `,
		identity.ID,
		identity.UserID,
		strings.ToLower(strings.TrimSpace(identity.Provider)),
		strings.TrimSpace(identity.ProviderUserID),
		nullableOAuthEmail(strings.ToLower(identity.ProviderEmail)),
		identity.EmailVerified,
		identity.CreatedAt,
		identity.UpdatedAt,
	).Scan(&identity.CreatedAt, &identity.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return sentinal_errors.ErrAlreadyExists
		}
		return err
	}

	return nil
}

func (r *PostgresOAuthIdentityRepository) GetByProviderSubject(ctx context.Context, provider, providerUserID string) (OAuthIdentity, error) {
	if strings.TrimSpace(provider) == "" || strings.TrimSpace(providerUserID) == "" {
		return OAuthIdentity{}, sentinal_errors.ErrInvalidInput
	}

	var identity OAuthIdentity
	var email sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, provider, provider_user_id, provider_email, email_verified, created_at, updated_at
        FROM oauth_identities
        WHERE provider = $1 AND provider_user_id = $2
    `, strings.ToLower(strings.TrimSpace(provider)), strings.TrimSpace(providerUserID)).Scan(
		&identity.ID,
		&identity.UserID,
		&identity.Provider,
		&identity.ProviderUserID,
		&email,
		&identity.EmailVerified,
		&identity.CreatedAt,
		&identity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OAuthIdentity{}, sentinal_errors.ErrNotFound
		}
		return OAuthIdentity{}, err
	}
	if email.Valid {
		identity.ProviderEmail = email.String
	}
	return identity, nil
}
