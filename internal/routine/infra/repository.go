package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db"
	"github.com/Adriano-silva131/planejo-backend/internal/routine/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

// adjustmentModel is the routine_adjustments row. The times are NULL when
// the occurrence is skipped.
type adjustmentModel struct {
	TaskID      uuid.UUID `gorm:"primaryKey"`
	Date        time.Time `gorm:"primaryKey"`
	UserID      uuid.UUID
	Skipped     bool
	StartMinute *int
	EndMinute   *int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (adjustmentModel) TableName() string {
	return "routine_adjustments"
}

func toModel(a domain.Adjustment) adjustmentModel {
	m := adjustmentModel{
		TaskID:    a.TaskID,
		Date:      a.Date.Time(),
		UserID:    a.UserID,
		Skipped:   a.Skipped,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
	if !a.Skipped {
		m.StartMinute, m.EndMinute = new(int(a.Start)), new(int(a.End))
	}
	return m
}

func toDomain(m adjustmentModel) domain.Adjustment {
	a := domain.Adjustment{
		UserID:    m.UserID,
		TaskID:    m.TaskID,
		Date:      calendar.NewDate(m.Date.Date()),
		Skipped:   m.Skipped,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
	if m.StartMinute != nil && m.EndMinute != nil {
		a.Start, a.End = calendar.TimeOfDay(*m.StartMinute), calendar.TimeOfDay(*m.EndMinute)
	}
	return a
}

type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Save upserts on (task_id, date). The update only applies to a row of the
// same user, so an adjustment of someone else is never overwritten.
func (r *Repository) Save(ctx context.Context, a domain.Adjustment) (domain.Adjustment, error) {
	m := toModel(a)
	res := db.Conn(ctx, r.db).Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_id"}, {Name: "date"}},
			Where:     clause.Where{Exprs: []clause.Expression{clause.Eq{Column: "routine_adjustments.user_id", Value: m.UserID}}},
			DoUpdates: clause.AssignmentColumns([]string{"skipped", "start_minute", "end_minute", "updated_at"}),
		},
		clause.Returning{},
	).Create(&m)
	if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
		return domain.Adjustment{}, domain.ErrTaskNotFound
	}
	if res.Error != nil {
		return domain.Adjustment{}, fmt.Errorf("routine: save adjustment: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.Adjustment{}, domain.ErrTaskNotFound
	}
	return toDomain(m), nil
}

func (r *Repository) Delete(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	res := db.Conn(ctx, r.db).
		Where("user_id = ? AND task_id = ? AND date = ?", userID, taskID, date.Time()).
		Delete(&adjustmentModel{})
	if res.Error != nil {
		return fmt.Errorf("routine: delete adjustment: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrAdjustmentNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]domain.Adjustment, error) {
	var models []adjustmentModel
	err := db.Conn(ctx, r.db).
		Where("user_id = ? AND date BETWEEN ? AND ?", userID, from.Time(), to.Time()).
		Order("date, task_id").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("routine: list adjustments: %w", err)
	}
	adjustments := make([]domain.Adjustment, 0, len(models))
	for _, m := range models {
		adjustments = append(adjustments, toDomain(m))
	}
	return adjustments, nil
}
