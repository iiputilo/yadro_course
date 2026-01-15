package db

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"log/slog"
	"os"
	"yadro.com/course/search/core"
)

func newTestDB(t *testing.T) (*DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}

	xdb := sqlx.NewDb(sqlDB, "sqlmock")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return &DB{log: logger, conn: xdb}, mock, func() {
		_ = sqlDB.Close()
	}
}

func TestSearchComics(t *testing.T) {
	db, mock, closeFn := newTestDB(t)
	defer closeFn()

	ctx := context.Background()
	words := []string{"foo", "bar"}
	limit := 2

	searchQuery := regexp.QuoteMeta(`
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
 `)

	rows := sqlmock.NewRows([]string{"id", "url"}).
		AddRow(1, "url1").
		AddRow(2, "url2")

	mock.ExpectQuery(searchQuery).
		WithArgs(sqlmock.AnyArg(), limit).
		WillReturnRows(rows)

	countQuery := regexp.QuoteMeta(`
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
 `)

	countRows := sqlmock.NewRows([]string{"count"}).AddRow(5)

	mock.ExpectQuery(countQuery).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(countRows)

	got, total, err := db.SearchComics(ctx, words, limit)
	if err != nil {
		t.Fatalf("SearchComics error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 comics, got %d", len(got))
	}
	if total != limit {
		t.Fatalf("expected total %d, got %d", limit, total)
	}
	if got[0].ID != 1 || got[0].URL != "url1" {
		t.Fatalf("unexpected first comic: %+v", got[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestLoadIndexData(t *testing.T) {
	db, mock, closeFn := newTestDB(t)
	defer closeFn()

	ctx := context.Background()

	query := regexp.QuoteMeta(`SELECT id, words FROM comics`)

	rows := sqlmock.NewRows([]string{"id", "words"}).
		AddRow(1, "{foo,bar}").
		AddRow(2, "{}").
		AddRow(3, "{baz}")

	mock.ExpectQuery(query).WillReturnRows(rows)

	data, err := db.LoadIndexData(ctx)
	if err != nil {
		t.Fatalf("LoadIndexData error: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("expected 3 records, got %d", len(data))
	}

	if len(data[1]) != 2 || data[1][0] != "foo" || data[1][1] != "bar" {
		t.Fatalf("unexpected data[1]: %#v", data[1])
	}

	if len(data[2]) != 0 {
		t.Fatalf("expected empty slice for id=2, got %#v", data[2])
	}

	if len(data[3]) != 1 || data[3][0] != "baz" {
		t.Fatalf("unexpected data[3]: %#v", data[3])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetComicsByIDs(t *testing.T) {
	db, mock, closeFn := newTestDB(t)
	defer closeFn()

	ctx := context.Background()
	ids := []int{2, 5, 7}

	queryRegex := `SELECT id, img_url as url FROM comics WHERE id IN \(.+\) ORDER BY id`

	rows := sqlmock.NewRows([]string{"id", "url"}).
		AddRow(2, "u2").
		AddRow(5, "u5").
		AddRow(7, "u7")

	mock.ExpectQuery(queryRegex).
		WithArgs(
			sql.NamedArg{Name: "", Value: 2},
			sql.NamedArg{Name: "", Value: 5},
			sql.NamedArg{Name: "", Value: 7},
		).
		WillReturnRows(rows)

	comics, err := db.GetComicsByIDs(ctx, ids)
	if err != nil {
		t.Fatalf("GetComicsByIDs error: %v", err)
	}

	if len(comics) != 3 {
		t.Fatalf("expected 3 comics, got %d", len(comics))
	}
	expected := []core.Comic{
		{ID: 2, URL: "u2"},
		{ID: 5, URL: "u5"},
		{ID: 7, URL: "u7"},
	}
	for i := range expected {
		if comics[i] != expected[i] {
			t.Fatalf("at %d expected %#v, got %#v", i, expected[i], comics[i])
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestGetComicsByIDs_Empty(t *testing.T) {
	db, _, closeFn := newTestDB(t)
	defer closeFn()

	ctx := context.Background()

	comics, err := db.GetComicsByIDs(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comics != nil {
		t.Fatalf("expected nil slice, got %#v", comics)
	}
}

func TestSearchComics_EmptyWords(t *testing.T) {
	db, _, closeFn := newTestDB(t)
	defer closeFn()

	ctx := context.Background()

	comics, total, err := db.SearchComics(ctx, nil, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comics != nil || total != 0 {
		t.Fatalf("expected nil,0 got %#v,%d", comics, total)
	}
}
