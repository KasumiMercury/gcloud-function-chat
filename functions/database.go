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

func getVideoRecordByStatus(ctx context.Context, db *bun.DB, status []string) ([]VideoRecord, error) {
	records := make([]VideoRecord, 0)
	err := db.NewSelect().Model(&records).Where("status IN (?)", status).Column("source_id", "status", "chat_id").Scan(ctx)
	if err != nil {
		return nil, err
	}

	return records, nil

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

func getLastPublishedAtOfRecordEachSource(ctx context.Context, db *bun.DB, source []string) (map[string]int64, error) {
	// Get the last recorded chat
	records := make([]ChatRecord, 0)
	err := db.NewSelect().
		Model(&records).
		ColumnExpr("source_id", "MAX(published_at) as published_at").
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
