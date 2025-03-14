// (c) Cartesi and individual authors (see AUTHORS)
// SPDX-License-Identifier: Apache-2.0 (see LICENSE)

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cartesi/rollups-node/internal/model"
	"github.com/cartesi/rollups-node/internal/repository"
	"github.com/cartesi/rollups-node/internal/repository/postgres/db/rollupsdb/public/table"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-jet/jet/v2/postgres"
)

func (r *PostgresRepository) GetInput(
	ctx context.Context,
	nameOrAddress string,
	inputIndex uint64,
) (*model.Input, error) {

	whereClause, err := getWhereClauseFromNameOrAddress(nameOrAddress)
	if err != nil {
		return nil, err
	}

	sel := table.Input.
		SELECT(
			table.Input.EpochApplicationID,
			table.Input.EpochIndex,
			table.Input.Index,
			table.Input.BlockNumber,
			table.Input.RawData,
			table.Input.Status,
			table.Input.MachineHash,
			table.Input.OutputsHash,
			table.Input.TransactionReference,
			table.Input.CreatedAt,
			table.Input.UpdatedAt,
		).
		FROM(
			table.Input.
				INNER_JOIN(table.Application,
					table.Input.EpochApplicationID.EQ(table.Application.ID),
				),
		).
		WHERE(
			whereClause.
				AND(table.Input.Index.EQ(postgres.RawFloat(fmt.Sprintf("%d", inputIndex)))),
		)

	sqlStr, args := sel.Sql()
	row := r.db.QueryRow(ctx, sqlStr, args...)

	var inp model.Input
	err = row.Scan(
		&inp.EpochApplicationID,
		&inp.EpochIndex,
		&inp.Index,
		&inp.BlockNumber,
		&inp.RawData,
		&inp.Status,
		&inp.MachineHash,
		&inp.OutputsHash,
		&inp.TransactionReference,
		&inp.CreatedAt,
		&inp.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inp, nil
}

func (r *PostgresRepository) GetInputByTxReference(
	ctx context.Context,
	nameOrAddress string,
	ref *common.Hash,
) (*model.Input, error) {

	if ref == nil {
		return nil, fmt.Errorf("tx reference is nil")
	}

	whereClause, err := getWhereClauseFromNameOrAddress(nameOrAddress)
	if err != nil {
		return nil, err
	}

	sel := table.Input.
		SELECT(
			table.Input.EpochApplicationID,
			table.Input.EpochIndex,
			table.Input.Index,
			table.Input.BlockNumber,
			table.Input.RawData,
			table.Input.Status,
			table.Input.MachineHash,
			table.Input.OutputsHash,
			table.Input.TransactionReference,
			table.Input.CreatedAt,
			table.Input.UpdatedAt,
		).
		FROM(
			table.Input.
				INNER_JOIN(table.Application,
					table.Input.EpochApplicationID.EQ(table.Application.ID),
				),
		).
		WHERE(
			whereClause.
				AND(table.Input.TransactionReference.EQ(postgres.Bytea(ref))),
		)

	sqlStr, args := sel.Sql()
	row := r.db.QueryRow(ctx, sqlStr, args...)

	var inp model.Input
	err = row.Scan(
		&inp.EpochApplicationID,
		&inp.EpochIndex,
		&inp.Index,
		&inp.BlockNumber,
		&inp.RawData,
		&inp.Status,
		&inp.MachineHash,
		&inp.OutputsHash,
		&inp.TransactionReference,
		&inp.CreatedAt,
		&inp.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inp, nil
}

func (r *PostgresRepository) GetLastInput(
	ctx context.Context,
	nameOrAddress string,
	epochIndex uint64,
) (*model.Input, error) {

	whereClause, err := getWhereClauseFromNameOrAddress(nameOrAddress)
	if err != nil {
		return nil, err
	}

	sel := table.Input.
		SELECT(
			table.Input.EpochApplicationID,
			table.Input.EpochIndex,
			table.Input.Index,
			table.Input.BlockNumber,
			table.Input.RawData,
			table.Input.Status,
			table.Input.MachineHash,
			table.Input.OutputsHash,
			table.Input.TransactionReference,
			table.Input.CreatedAt,
			table.Input.UpdatedAt,
		).
		FROM(
			table.Input.
				INNER_JOIN(table.Application,
					table.Input.EpochApplicationID.EQ(table.Application.ID),
				),
		).
		WHERE(
			whereClause.
				AND(table.Input.EpochIndex.EQ(postgres.RawFloat(fmt.Sprintf("%d", epochIndex)))),
		).
		ORDER_BY(table.Input.Index.DESC()).
		LIMIT(1)

	sqlStr, args := sel.Sql()
	row := r.db.QueryRow(ctx, sqlStr, args...)

	var inp model.Input
	err = row.Scan(
		&inp.EpochApplicationID,
		&inp.EpochIndex,
		&inp.Index,
		&inp.BlockNumber,
		&inp.RawData,
		&inp.Status,
		&inp.MachineHash,
		&inp.OutputsHash,
		&inp.TransactionReference,
		&inp.CreatedAt,
		&inp.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inp, nil
}

func (r *PostgresRepository) ListInputs(
	ctx context.Context,
	nameOrAddress string,
	f repository.InputFilter,
	p repository.Pagination,
) ([]*model.Input, uint64, error) {

	whereClause, err := getWhereClauseFromNameOrAddress(nameOrAddress)
	if err != nil {
		return nil, 0, err
	}

	sel := table.Input.
		SELECT(
			table.Input.EpochApplicationID,
			table.Input.EpochIndex,
			table.Input.Index,
			table.Input.BlockNumber,
			table.Input.RawData,
			table.Input.Status,
			table.Input.MachineHash,
			table.Input.OutputsHash,
			table.Input.TransactionReference,
			table.Input.CreatedAt,
			table.Input.UpdatedAt,
			postgres.COUNT(postgres.STAR).OVER().AS("total_count"),
		).
		FROM(
			table.Input.
				INNER_JOIN(table.Application,
					table.Input.EpochApplicationID.EQ(table.Application.ID),
				),
		)

	conditions := []postgres.BoolExpression{whereClause}
	if f.EpochIndex != nil {
		conditions = append(conditions, table.Input.EpochIndex.EQ(postgres.RawFloat(fmt.Sprintf("%d", *f.EpochIndex))))
	}

	if f.Status != nil {
		conditions = append(conditions, table.Input.Status.EQ(postgres.NewEnumValue(f.Status.String())))
	}

	if f.NotStatus != nil {
		conditions = append(conditions, table.Input.Status.NOT_EQ(postgres.NewEnumValue(f.NotStatus.String())))
	}

	if f.Sender != nil {
		conditions = append(conditions,
			postgres.SUBSTR(table.Input.RawData, postgres.Int(81), postgres.Int(20)).EQ(postgres.Bytea(f.Sender.Bytes())),
		)
	}

	sel = sel.WHERE(postgres.AND(conditions...)).ORDER_BY(table.Input.Index.ASC())

	if p.Limit > 0 {
		sel = sel.LIMIT(int64(p.Limit))
	}
	if p.Offset > 0 {
		sel = sel.OFFSET(int64(p.Offset))
	}

	sqlStr, args := sel.Sql()
	rows, err := r.db.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var inputs []*model.Input
	var total uint64
	for rows.Next() {
		var in model.Input
		err := rows.Scan(
			&in.EpochApplicationID,
			&in.EpochIndex,
			&in.Index,
			&in.BlockNumber,
			&in.RawData,
			&in.Status,
			&in.MachineHash,
			&in.OutputsHash,
			&in.TransactionReference,
			&in.CreatedAt,
			&in.UpdatedAt,
			&total,
		)
		if err != nil {
			return nil, 0, err
		}
		inputs = append(inputs, &in)
	}
	return inputs, total, nil
}
