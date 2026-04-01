package listing

import (
	"fmt"

	"github.com/jackc/pgx/v5"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanRow(row scanner) (Listing, error) {
	var l Listing
	var typ, status string
	err := row.Scan(
		&l.ID, &l.TenantID, &l.PackageID, &l.Name, &l.Slug, &l.Description,
		&typ, &l.Category, &status, &l.AvgRating, &l.ReviewCount, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}
	l.Type = PackageType(typ)
	l.Status = ListingStatus(status)
	return l, nil
}

func scanRows(rows pgx.Rows) ([]Listing, error) {
	var listings []Listing
	for rows.Next() {
		var l Listing
		var typ, status string
		if err := rows.Scan(
			&l.ID, &l.TenantID, &l.PackageID, &l.Name, &l.Slug, &l.Description,
			&typ, &l.Category, &status, &l.AvgRating, &l.ReviewCount, &l.CreatedAt, &l.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("listing: scan row: %w", err)
		}
		l.Type = PackageType(typ)
		l.Status = ListingStatus(status)
		listings = append(listings, l)
	}
	return listings, rows.Err()
}
