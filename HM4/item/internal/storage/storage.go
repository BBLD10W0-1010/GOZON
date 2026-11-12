package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BBLD10W0-1010/GOZON/HM4/item/internal/domain"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

// OldestInCity returns the oldest user, breaking age ties by name, then ID.
// If the city has no users, the error matches pgx.ErrNoRows.
func (s *Storage) OldestInCity(ctx context.Context, city string) (domain.User, error) {
	const query = `
		SELECT id, name, age, city
		FROM users
		WHERE city = $1
		  AND age = (SELECT MAX(age) FROM users WHERE city = $1)
		ORDER BY name, id
		LIMIT 1
	`

	var user domain.User
	err := s.pool.QueryRow(ctx, query, city).Scan(&user.ID, &user.Name, &user.Age, &user.City)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("no users in city %q: %w", city, err)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("query oldest user in city %q: %w", city, err)
	}
	return user, nil
}
