package functions

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

func NewDBClient(dsn string) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	db := bun.NewDB(sqldb, pgdialect.New())
	return db, nil
}

func getVideoRecordByStatus(ctx context.Context, db *bun.DB, status []string) ([]VideoRecord, error) {
	records := make([]VideoRecord, 0)
	err := db.NewSelect().Model(&records).Where("status IN (?)", bun.In(status)).Column("source_id", "status", "chat_id").Scan(ctx)
	if err != nil {
		return nil, err
	}

	return records, nil

}

func getLastPublishedAtOfRecordEachSource(ctx context.Context, db *bun.DB, source []string) (map[string]int64, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("source is empty")
	}
	// Get the last recorded chat
	records := make([]ChatRecord, 0)
	err := db.NewSelect().
		Model(&records).
		ColumnExpr("source_id, MAX(published_at) as published_at").
		Where("source_id IN (?)", source).
		Group("source_id").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int64)
	for _, record := range records {
		result[record.SourceID] = record.PublishedAt.Unix()
	}

	return result, nil
}

func InsertChatRecord(ctx context.Context, db *bun.DB, record []ChatRecord) error {
	_, err := db.NewInsert().Model(&record).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}
