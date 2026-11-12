package storage_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BBLD10W0-1010/GOZON/HM4/item/internal/domain"
	"github.com/BBLD10W0-1010/GOZON/HM4/item/internal/storage"
)

func TestOldestInCity(t *testing.T) {
	dsn := os.Getenv("HM4_TEST_DSN")
	if dsn == "" {
		t.Skip("set HM4_TEST_DSN to a PostgreSQL connection string")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1 // The temporary table belongs to this connection.
	config.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `CREATE TEMP TABLE users (
		id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, age INT NOT NULL, city TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO users (name, age, city) VALUES
		('Vera', 40, 'Moscow'), ('Boris', 40, 'Moscow'), ('Boris', 40, 'Moscow'),
		('Anna', 25, 'Moscow'), ('Gleb', 50, 'Kazan')`)
	if err != nil {
		t.Fatal(err)
	}

	s := storage.New(pool)
	got, err := s.OldestInCity(ctx, "Moscow")
	want := domain.User{ID: 2, Name: "Boris", Age: 40, City: "Moscow"}
	if err != nil || got != want {
		t.Fatalf("OldestInCity(Moscow) = (%+v, %v), want (%+v, nil)", got, err, want)
	}

	got, err = s.OldestInCity(ctx, "Missing")
	if got != (domain.User{}) || !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("OldestInCity(Missing) = (%+v, %v), want (zero user, pgx.ErrNoRows)", got, err)
	}
}
