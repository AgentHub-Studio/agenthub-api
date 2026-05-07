package oauth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Repository handles persistence of OAuthCredential entities.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ListAll returns a paginated list of credentials for the given tenant.
func (r *Repository) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]OAuthCredential, int, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, 0, err
	}
	defer release()

	var total int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_credential`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("oauth: count: %w", err)
	}

	rows, err := conn.Query(ctx,
		`SELECT id, name, auth_type, token_url, client_id, client_secret, scopes,
		        api_key_header, api_key_value, api_key_location, bearer_token, username, password,
		        auth_url, redirect_url, code_verifier, refresh_token, expires_at,
		        created_at, updated_at
		 FROM oauth_credential
		 ORDER BY name
		 LIMIT $1 OFFSET $2`,
		pr.Size, pr.Offset(),
	)
	if err != nil {
		return nil, 0, fmt.Errorf("oauth: list: %w", err)
	}
	defer rows.Close()
	
	var items []OAuthCredential
	for rows.Next() {
		var c OAuthCredential
		if err := rows.Scan(
			&c.ID, &c.Name, &c.AuthType, &c.TokenURL, &c.ClientID, &c.ClientSecret,
			&c.Scopes, &c.APIKeyHeader, &c.APIKeyValue, &c.APIKeyLocation, &c.BearerToken, &c.Username,
			&c.Password, &c.AuthURL, &c.RedirectURL, &c.CodeVerifier, &c.RefreshToken,
			&c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("oauth: scan: %w", err)
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}

// GetByID retrieves a credential by ID for the given tenant.
func (r *Repository) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (OAuthCredential, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return OAuthCredential{}, err
	}
	defer release()

	var c OAuthCredential
	err = conn.QueryRow(ctx,
		`SELECT id, name, auth_type, token_url, client_id, client_secret, scopes,
		        api_key_header, api_key_value, api_key_location, bearer_token, username, password,
		        auth_url, redirect_url, code_verifier, refresh_token, expires_at,
		        created_at, updated_at
		 FROM oauth_credential WHERE id = $1`,
		id,
	).Scan(
		&c.ID, &c.Name, &c.AuthType, &c.TokenURL, &c.ClientID, &c.ClientSecret,
		&c.Scopes, &c.APIKeyHeader, &c.APIKeyValue, &c.BearerToken, &c.Username,
		&c.Password, &c.AuthURL, &c.RedirectURL, &c.CodeVerifier, &c.RefreshToken,
		&c.ExpiresAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthCredential{}, ErrNotFound
	}
	return c, err
}

// Create inserts a new credential for the given tenant.
func (r *Repository) Create(ctx context.Context, tenantID string, c OAuthCredential) (OAuthCredential, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return OAuthCredential{}, err
	}
	defer release()

	var created OAuthCredential
	err = conn.QueryRow(ctx,
		`INSERT INTO oauth_credential
		 (name, auth_type, token_url, client_id, client_secret, scopes,
		  api_key_header, api_key_value, api_key_location, bearer_token, username, password,
		  auth_url, redirect_url, code_verifier)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 RETURNING id, name, auth_type, token_url, client_id, client_secret, scopes,
		           api_key_header, api_key_value, api_key_location, bearer_token, username, password,
		           auth_url, redirect_url, code_verifier, refresh_token, expires_at,
		           created_at, updated_at`,
		c.Name, c.AuthType, c.TokenURL, c.ClientID, c.ClientSecret, c.Scopes,
		c.APIKeyHeader, c.APIKeyValue, c.APIKeyLocation, c.BearerToken, c.Username, c.Password,
		c.AuthURL, c.RedirectURL, c.CodeVerifier,
	).Scan(
		&created.ID, &created.Name, &created.AuthType, &created.TokenURL, &created.ClientID,
		&created.ClientSecret, &created.Scopes, &created.APIKeyHeader, &created.APIKeyValue, &created.APIKeyLocation,
		&created.BearerToken, &created.Username, &created.Password,
		&created.AuthURL, &created.RedirectURL, &created.CodeVerifier, &created.RefreshToken,
		&created.ExpiresAt, &created.CreatedAt, &created.UpdatedAt,
	)
	return created, err
}

// Update updates an existing credential for the given tenant.
func (r *Repository) Update(ctx context.Context, tenantID string, id uuid.UUID, c OAuthCredential) (OAuthCredential, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return OAuthCredential{}, err
	}
	defer release()

	var updated OAuthCredential
	err = conn.QueryRow(ctx,
		`UPDATE oauth_credential SET
		   name=$1, auth_type=$2, token_url=$3, client_id=$4, client_secret=$5, scopes=$6,
		   api_key_header=$7, api_key_value=$8, api_key_location=$9, bearer_token=$10, username=$11, password=$12,
		   auth_url=$13, redirect_url=$14, code_verifier=$15, refresh_token=$16, expires_at=$17,
		   updated_at=NOW()
		 WHERE id=$18
		 RETURNING id, name, auth_type, token_url, client_id, client_secret, scopes,
		           api_key_header, api_key_value, api_key_location, bearer_token, username, password,
		           auth_url, redirect_url, code_verifier, refresh_token, expires_at,
		           created_at, updated_at`,
		c.Name, c.AuthType, c.TokenURL, c.ClientID, c.ClientSecret, c.Scopes,
		c.APIKeyHeader, c.APIKeyValue, c.APIKeyLocation, c.BearerToken, c.Username, c.Password,
		c.AuthURL, c.RedirectURL, c.CodeVerifier, c.RefreshToken, c.ExpiresAt, id,
	).Scan(
		&updated.ID, &updated.Name, &updated.AuthType, &updated.TokenURL, &updated.ClientID,
		&updated.ClientSecret, &updated.Scopes, &updated.APIKeyHeader, &updated.APIKeyValue, &updated.APIKeyLocation,
		&updated.BearerToken, &updated.Username, &updated.Password,
		&updated.AuthURL, &updated.RedirectURL, &updated.CodeVerifier, &updated.RefreshToken,
		&updated.ExpiresAt, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthCredential{}, ErrNotFound
	}
	return updated, err
}

// Delete removes a credential by ID for the given tenant.
func (r *Repository) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return err
	}
	defer release()

	ct, err := conn.Exec(ctx, `DELETE FROM oauth_credential WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("oauth: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
