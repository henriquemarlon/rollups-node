// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/cartesi/rollups-node/internal/repository/postgres/db/rollupsdb/public/table"

	"github.com/go-jet/jet/v2/postgres"
)

func (r *PostgresRepository) SaveNodeConfigRaw(ctx context.Context, key string, rawJSON []byte) error {

	insertStmt := table.NodeConfig.
		INSERT(
			table.NodeConfig.Key,
			table.NodeConfig.Value,
		).
		VALUES(
			key,
			postgres.Json(rawJSON),
		).
		ON_CONFLICT(table.NodeConfig.Key).
		DO_UPDATE(postgres.SET(table.NodeConfig.Value.SET(postgres.Json(rawJSON))))

	sqlStr, args := insertStmt.Sql()
	_, err := r.db.Exec(ctx, sqlStr, args...)
	return err
}

func (r *PostgresRepository) LoadNodeConfigRaw(ctx context.Context, key string) ([]byte, time.Time, time.Time, error) {
	sel := table.NodeConfig.
		SELECT(
			table.NodeConfig.Value,
			table.NodeConfig.CreatedAt,
			table.NodeConfig.UpdatedAt,
		).
		WHERE(table.NodeConfig.Key.EQ(postgres.String(key))).
		LIMIT(1)

	sqlStr, args := sel.Sql()
	row := r.db.QueryRow(ctx, sqlStr, args...)

	var (
		value     []byte
		createdAt time.Time
		updatedAt time.Time
	)
	err := row.Scan(
		&value,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	return value, createdAt, updatedAt, nil
}
