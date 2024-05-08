package kitchen

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PipelineSqlTxCookware[M IPipelineModel] struct {
	db *sql.DB
}

func NewPipelineSqlTxCookware[M IPipelineModel](db *sql.DB) *PipelineSqlTxCookware[M] {
	return &PipelineSqlTxCookware[M]{db: db}
}

func (p PipelineSqlTxCookware[M]) BeginTx(ctx context.Context, opts ...*sql.TxOptions) (IDbTx, error) {
	if len(opts) == 0 {
		return p.db.BeginTx(ctx, nil)
	} else {
		return p.db.BeginTx(ctx, opts[0])
	}
}

func (p PipelineSqlTxCookware[M]) FinishTx(tx IDbTx, err error) error {
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.New(fmt.Sprintf("tx rollback error: %v, original error: %v", rollbackErr, err))
		}
		return err
	} else {
		return tx.Commit()
	}
	return err
}

type DepositOrder struct {
	Id     int
	Status PipelineStatus
}

func (p DepositOrder) PrimaryKey() any {
	return p.Id
}

func (p PipelineSqlTxCookware[M]) GetModelById(ctx context.Context, id ...any) (model M, err error) {
	return any(&DepositOrder{Status: "Pending"}).(M), nil
}

func (p PipelineSqlTxCookware[M]) SaveModel(db IDbRunner, model M, oStatus PipelineStatus) error {
	fmt.Println("save model", model)
	return nil
}
