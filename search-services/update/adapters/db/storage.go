package db

import (
	"context"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"yadro.com/course/update/core"
)

type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {
	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}
	return &DB{log: log, conn: db}, nil
}

func (db *DB) Add(ctx context.Context, comics core.Comics) error {
	words := comics.Words
	if words == nil {
		words = []string{}
	}

	_, err := db.conn.ExecContext(
		ctx,
		`INSERT INTO comics (id, img_url, words)
         VALUES ($1, $2, $3::text[])
         ON CONFLICT (id) DO NOTHING`,
		comics.ID,
		comics.URL,
		words,
	)
	return err
}

func (db *DB) Stats(ctx context.Context) (core.DBStats, error) {
	var st core.DBStats
	if err := db.conn.GetContext(ctx, &st.WordsTotal,
		`SELECT COALESCE(SUM(array_length(words, 1)), 0) FROM comics`,
	); err != nil {
		return core.DBStats{}, err
	}
	if err := db.conn.GetContext(ctx, &st.WordsUnique,
		`SELECT COALESCE(COUNT(DISTINCT w), 0)
         FROM comics, UNNEST(words) AS w`,
	); err != nil {
		return core.DBStats{}, err
	}
	if err := db.conn.GetContext(ctx, &st.ComicsFetched,
		`SELECT COUNT(*) FROM comics`,
	); err != nil {
		return core.DBStats{}, err
	}
	return st, nil
}

func (db *DB) IDs(ctx context.Context) ([]int, error) {
	var ids []int
	if err := db.conn.SelectContext(ctx, &ids, `SELECT id FROM comics ORDER BY id`); err != nil {
		return nil, err
	}
	return ids, nil
}

func (db *DB) Drop(ctx context.Context) error {
	_, err := db.conn.ExecContext(ctx, `TRUNCATE TABLE comics`)
	return err
}
