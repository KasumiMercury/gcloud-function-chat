package functions

import (
	"context"
	"database/sql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func NewDBClient(dsn string) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqldb, pgdialect.New())
	return db, nil
}

func getLastPublishedAtOfRecord(ctx context.Context, db *bun.DB) (int64, error) {
	// Get the last recorded chat
	record := new(ChatRecord)
	err := db.NewSelect().Model(record).Order("published_at DESC").Limit(1).Column("published_at").Scan(ctx)
	if err != nil {
		return 0, err
	}

	if record == nil {
		return 0, nil
	}

	// Get the last published_at
	lastPublishedAt := record.PublishedAt.Unix()

	return lastPublishedAt, nil
}

func InsertChatRecord(ctx context.Context, db *bun.DB, record []ChatRecord) error {
	_, err := db.NewInsert().Model(&record).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}
