package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

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

var upsert = db.OwnedUpsert{
	Table:    "routine_adjustments",
	Conflict: []string{"task_id", "date"},
	Update:   []string{"skipped", "start_minute", "end_minute", "updated_at"},
}

func (r *Repository) Save(ctx context.Context, a domain.Adjustment) (domain.Adjustment, error) {
	m := toModel(a)
	err := upsert.Save(ctx, r.db, &m, m.UserID)
	switch {
	case errors.Is(err, db.ErrNotOwned), errors.Is(err, gorm.ErrForeignKeyViolated):
		return domain.Adjustment{}, domain.ErrTaskNotFound
	case err != nil:
		return domain.Adjustment{}, fmt.Errorf("routine: save adjustment: %w", err)
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
	return find(db.Conn(ctx, r.db).
		Where("user_id = ? AND date BETWEEN ? AND ?", userID, from.Time(), to.Time()))
}

func (r *Repository) ForTask(ctx context.Context, userID, taskID uuid.UUID) ([]domain.Adjustment, error) {
	return find(db.Conn(ctx, r.db).Where("user_id = ? AND task_id = ?", userID, taskID))
}

func find(q *gorm.DB) ([]domain.Adjustment, error) {
	var models []adjustmentModel
	if err := q.Order("date, task_id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("routine: list adjustments: %w", err)
	}
	adjustments := make([]domain.Adjustment, 0, len(models))
	for _, m := range models {
		adjustments = append(adjustments, toDomain(m))
	}
	return adjustments, nil
}
