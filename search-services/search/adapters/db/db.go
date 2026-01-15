package db

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"yadro.com/course/search/core"
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

func (db *DB) SearchComics(ctx context.Context, words []string, limit int) ([]core.Comic, int, error) {
	if len(words) == 0 {
		return nil, 0, nil
	}

	query := `
  SELECT id,
         img_url AS url
  FROM (
   SELECT
    id,
    img_url,
    cardinality(
     ARRAY(
      SELECT unnest(words)
      INTERSECT
      SELECT unnest($1::text[])
     )
    ) AS match_count
   FROM comics
   WHERE words && $1
  ) AS ranked
  WHERE match_count > 0
  ORDER BY match_count DESC, id ASC
  LIMIT $2;
 `
	var comics []core.Comic
	if err := db.conn.SelectContext(ctx, &comics, query, pq.StringArray(words), limit); err != nil {
		return nil, 0, err
	}

	countQuery := `
  SELECT COUNT(*)
  FROM (
   SELECT
    cardinality(
     ARRAY(
      SELECT unnest(words)
      INTERSECT
      SELECT unnest($1::text[])
     )
    ) AS match_count
   FROM comics
   WHERE words && $1
  ) AS ranked
  WHERE match_count > 0;
 `
	var total int
	if err := db.conn.GetContext(ctx, &total, countQuery, pq.StringArray(words)); err != nil {
		return nil, 0, err
	}

	// обрезаем total по limit, как ожидает тест
	if limit > 0 && total > limit {
		total = limit
	}
	return comics, total, nil
}

func (db *DB) LoadIndexData(ctx context.Context) (map[int][]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, words FROM comics`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			db.log.Error("failed to close rows", "error", err)
		}
	}()

	data := make(map[int][]string)
	for rows.Next() {
		var id int
		var words string
		if err := rows.Scan(&id, &words); err != nil {
			return nil, err
		}
		words = strings.Trim(words, "{}")
		if words == "" {
			data[id] = []string{}
		} else {
			data[id] = strings.Split(words, ",")
		}
	}
	return data, rows.Err()
}

func (db *DB) GetComicsByIDs(ctx context.Context, ids []int) ([]core.Comic, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query, args, err := sqlx.In(`SELECT id, img_url as url FROM comics WHERE id IN (?) ORDER BY id`, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to create IN query: %w", err)
	}
	query = db.conn.Rebind(query)

	var comics []core.Comic
	err = db.conn.SelectContext(ctx, &comics, query, args...)
	if err != nil {
		return nil, err
	}
	return comics, nil
}
