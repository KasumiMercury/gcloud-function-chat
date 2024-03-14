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

func InsertChatRecord(ctx context.Context, db *bun.DB, record []ChatRecord) error {
	_, err := db.NewInsert().Model(&record).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}
