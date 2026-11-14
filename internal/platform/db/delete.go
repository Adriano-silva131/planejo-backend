package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskDay is the key of a row kept per task and day, owned by a user.
type TaskDay struct {
	UserID uuid.UUID
	TaskID uuid.UUID
	Date   time.Time
}

// DeleteTaskDay deletes the row of model's table at key, reporting whether
// there was one. A row of another user is never deleted.
func DeleteTaskDay(ctx context.Context, conn *gorm.DB, model any, key TaskDay) (bool, error) {
	res := Conn(ctx, conn).
		Where("user_id = ? AND task_id = ? AND date = ?", key.UserID, key.TaskID, key.Date).
		Delete(model)
	return res.RowsAffected > 0, res.Error
}
