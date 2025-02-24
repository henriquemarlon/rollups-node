// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package postgres

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/jackc/pgx/v5"

	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository/postgres/db/rollupsdb/public/table"
)

var (
	ErrNoUpdate = fmt.Errorf("update did not take effect")
)

// Retrieve the claim of each application with the smallest index.
// The query may return either 0 or 1 entries per application.
func (r *PostgresRepository) selectOldestClaimPerApp(
	ctx context.Context,
	epochStatus model.EpochStatus,
) (
	map[common.Address]*model.ClaimRow,
	error,
) {
	if (epochStatus != model.EpochStatus_ClaimSubmitted) && (epochStatus != model.EpochStatus_ClaimComputed) {
		return nil, fmt.Errorf("Invalid epoch status: %v", epochStatus)
	}

	// NOTE(mpolitzer): DISTINCT ON is a postgres extension. To implement
	// this in SQLite there is an alternative using GROUP BY and HAVING
	// clauses instead.
	stmt := table.Epoch.SELECT(
		table.Epoch.ApplicationID,
		table.Epoch.Index,
		table.Epoch.FirstBlock,
		table.Epoch.LastBlock,
		table.Epoch.ClaimHash,
		table.Epoch.ClaimTransactionHash,
		table.Epoch.Status,
		table.Epoch.VirtualIndex,
		table.Epoch.CreatedAt,
		table.Epoch.UpdatedAt,
		table.Application.IapplicationAddress,
		table.Application.IconsensusAddress,
	).
		DISTINCT(table.Epoch.ApplicationID).
		FROM(
			table.Epoch.
				INNER_JOIN(
					table.Application,
					table.Epoch.ApplicationID.EQ(table.Application.ID),
				),
		).
		WHERE(table.Epoch.Status.EQ(postgres.NewEnumValue(epochStatus.String())).AND(table.Application.State.EQ(postgres.NewEnumValue(model.ApplicationState_Enabled.String())))).
		ORDER_BY(
			table.Epoch.ApplicationID,
			table.Epoch.Index.ASC(),
		)

	sqlStr, args := stmt.Sql()
	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := map[common.Address]*model.ClaimRow{}
	for rows.Next() {
		var cr model.ClaimRow
		err := rows.Scan(
			&cr.ApplicationID,
			&cr.Index,
			&cr.FirstBlock,
			&cr.LastBlock,
			&cr.ClaimHash,
			&cr.ClaimTransactionHash,
			&cr.Status,
			&cr.VirtualIndex,
			&cr.CreatedAt,
			&cr.UpdatedAt,
			&cr.IApplicationAddress,
			&cr.IConsensusAddress,
		)
		if err != nil {
			return nil, err
		}
		results[cr.IApplicationAddress] = &cr
	}
	return results, nil
}

// Retrieve the newest accepted claim of each application
func (r *PostgresRepository) selectNewestAcceptedClaimPerApp(
	ctx context.Context,
	includeSubmitted bool,
) (
	map[common.Address]*model.ClaimRow,
	error,
) {
	expr := table.Epoch.Status.EQ(postgres.NewEnumValue(model.EpochStatus_ClaimAccepted.String()))
	if includeSubmitted {
		expr = expr.OR(table.Epoch.Status.EQ(postgres.NewEnumValue(model.EpochStatus_ClaimSubmitted.String())))
	}

	// NOTE(mpolitzer): DISTINCT ON is a postgres extension. To implement
	// this in SQLite there is an alternative using GROUP BY and HAVING
	// clauses instead.
	stmt := table.Epoch.SELECT(
		table.Epoch.ApplicationID,
		table.Epoch.Index,
		table.Epoch.FirstBlock,
		table.Epoch.LastBlock,
		table.Epoch.ClaimHash,
		table.Epoch.ClaimTransactionHash,
		table.Epoch.Status,
		table.Epoch.VirtualIndex,
		table.Epoch.CreatedAt,
		table.Epoch.UpdatedAt,
		table.Application.IapplicationAddress,
		table.Application.IconsensusAddress,
	).
		DISTINCT(table.Epoch.ApplicationID).
		FROM(
			table.Epoch.
				INNER_JOIN(
					table.Application,
					table.Epoch.ApplicationID.EQ(table.Application.ID),
				),
		).
		WHERE(expr.AND(table.Application.State.EQ(postgres.NewEnumValue(model.ApplicationState_Enabled.String())))).
		ORDER_BY(
			table.Epoch.ApplicationID,
			table.Epoch.Index.DESC(),
		)

	sqlStr, args := stmt.Sql()
	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := map[common.Address]*model.ClaimRow{}
	for rows.Next() {
		var cr model.ClaimRow
		err := rows.Scan(
			&cr.ApplicationID,
			&cr.Index,
			&cr.FirstBlock,
			&cr.LastBlock,
			&cr.ClaimHash,
			&cr.ClaimTransactionHash,
			&cr.Status,
			&cr.VirtualIndex,
			&cr.CreatedAt,
			&cr.UpdatedAt,
			&cr.IApplicationAddress,
			&cr.IConsensusAddress,
		)
		if err != nil {
			return nil, err
		}
		results[cr.IApplicationAddress] = &cr
	}
	return results, nil
}

func (r *PostgresRepository) SelectSubmissionClaimPairsPerApp(ctx context.Context) (
	map[common.Address]*model.ClaimRow,
	map[common.Address]*model.ClaimRow,
	error,
) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Commit(ctx)

	computed, err := r.selectOldestClaimPerApp(ctx, model.EpochStatus_ClaimComputed)
	if err != nil {
		return nil, nil, err
	}

	acceptedOrSubmitted, err := r.selectNewestAcceptedClaimPerApp(ctx, true)
	if err != nil {
		return nil, nil, err
	}

	return acceptedOrSubmitted, computed, err
}

func (r *PostgresRepository) SelectAcceptanceClaimPairsPerApp(ctx context.Context) (
	map[common.Address]*model.ClaimRow,
	map[common.Address]*model.ClaimRow,
	error,
) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Commit(ctx)

	submitted, err := r.selectOldestClaimPerApp(ctx, model.EpochStatus_ClaimSubmitted)
	if err != nil {
		return nil, nil, err
	}

	accepted, err := r.selectNewestAcceptedClaimPerApp(ctx, false)
	if err != nil {
		return nil, nil, err
	}

	return accepted, submitted, err
}

func (r *PostgresRepository) UpdateEpochWithSubmittedClaim(
	ctx context.Context,
	application_id int64,
	index uint64,
	transaction_hash common.Hash,
) error {
	updStmt := table.Epoch.
		UPDATE(
			table.Epoch.ClaimTransactionHash,
			table.Epoch.Status,
		).
		SET(
			transaction_hash,
			postgres.NewEnumValue(model.EpochStatus_ClaimSubmitted.String()),
		).
		FROM(
			table.Application,
		).
		WHERE(
			table.Epoch.ApplicationID.EQ(postgres.Int64(application_id)).
				AND(table.Epoch.Index.EQ(postgres.RawFloat(fmt.Sprintf("%d", index)))).
				AND(table.Epoch.Status.EQ(postgres.NewEnumValue(model.EpochStatus_ClaimComputed.String()))),
		)

	sqlStr, args := updStmt.Sql()
	cmd, err := r.db.Exec(ctx, sqlStr, args...)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNoUpdate
	}
	return nil
}

func (r *PostgresRepository) UpdateEpochWithAcceptedClaim(
	ctx context.Context,
	application_id int64,
	index uint64,
) error {
	updStmt := table.Epoch.
		UPDATE(
			table.Epoch.Status,
		).
		SET(
			postgres.NewEnumValue(model.EpochStatus_ClaimAccepted.String()),
		).
		FROM(
			table.Application,
		).
		WHERE(
			table.Epoch.ApplicationID.EQ(postgres.Int64(application_id)).
				AND(table.Epoch.Index.EQ(postgres.RawFloat(fmt.Sprintf("%d", index)))).
				AND(table.Epoch.Status.EQ(postgres.NewEnumValue(model.EpochStatus_ClaimSubmitted.String()))),
		)

	sqlStr, args := updStmt.Sql()
	cmd, err := r.db.Exec(ctx, sqlStr, args...)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNoUpdate
	}
	return nil
}
