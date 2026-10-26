package db

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

type Transactor struct {
	db *gorm.DB
}

func NewTransactor(db *gorm.DB) Transactor {
	return Transactor{db: db}
}

// InTx runs fn in a transaction carried by ctx: repositories that get their
// connection through Conn join it, and a nested InTx becomes a savepoint.
func (t Transactor) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return Conn(ctx, t.db).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Conn returns the transaction carried by ctx, or db when there is none, bound to ctx.
func Conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}
