package db

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotOwned reports that the row an upsert conflicted with belongs to
// another user.
var ErrNotOwned = errors.New("db: row belongs to another user")

// OwnedUpsert writes rows that carry a user_id. On a conflict on the Conflict
// columns it updates the Update columns, but only when the existing row
// belongs to the same user, so a row of someone else is never overwritten.
type OwnedUpsert struct {
	Table    string
	Conflict []string
	Update   []string
}

// Save writes row, a pointer to the model, for userID and reads it back. It
// fails with ErrNotOwned when the conflicting row belongs to another user; a
// broken foreign key comes as gorm.ErrForeignKeyViolated.
func (u OwnedUpsert) Save(ctx context.Context, conn *gorm.DB, row any, userID uuid.UUID) error {
	columns := make([]clause.Column, 0, len(u.Conflict))
	for _, name := range u.Conflict {
		columns = append(columns, clause.Column{Name: name})
	}
	res := Conn(ctx, conn).Clauses(
		clause.OnConflict{
			Columns:   columns,
			Where:     clause.Where{Exprs: []clause.Expression{clause.Eq{Column: u.Table + ".user_id", Value: userID}}},
			DoUpdates: clause.AssignmentColumns(u.Update),
		},
		clause.Returning{},
	).Create(row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotOwned
	}
	return nil
}
