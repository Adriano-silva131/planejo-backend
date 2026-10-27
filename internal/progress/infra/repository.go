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
	"github.com/Adriano-silva131/planejo-backend/internal/progress/domain"
	"github.com/Adriano-silva131/planejo-backend/internal/shared/calendar"
)

type checkInModel struct {
	TaskID    uuid.UUID `gorm:"primaryKey"`
	Date      time.Time `gorm:"primaryKey"`
	UserID    uuid.UUID
	Minutes   int
	Completed bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (checkInModel) TableName() string {
	return "checkins"
}

func toModel(c domain.CheckIn) checkInModel {
	return checkInModel{
		TaskID:    c.TaskID,
		Date:      c.Date.Time(),
		UserID:    c.UserID,
		Minutes:   c.Minutes,
		Completed: c.Completed,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func toDomain(m checkInModel) domain.CheckIn {
	return domain.CheckIn{
		UserID:    m.UserID,
		TaskID:    m.TaskID,
		Date:      calendar.NewDate(m.Date.Date()),
		Minutes:   m.Minutes,
		Completed: m.Completed,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Save upserts on (task_id, date). The update only applies to a row of the
// same user, so a check-in of someone else is never overwritten.
func (r *Repository) Save(ctx context.Context, c domain.CheckIn) (domain.CheckIn, error) {
	m := toModel(c)
	res := db.Conn(ctx, r.db).Clauses(
		clause.OnConflict{
			Columns:   []clause.Column{{Name: "task_id"}, {Name: "date"}},
			Where:     clause.Where{Exprs: []clause.Expression{clause.Eq{Column: "checkins.user_id", Value: m.UserID}}},
			DoUpdates: clause.AssignmentColumns([]string{"minutes", "completed", "updated_at"}),
		},
		clause.Returning{},
	).Create(&m)
	if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
		return domain.CheckIn{}, domain.ErrTaskNotFound
	}
	if res.Error != nil {
		return domain.CheckIn{}, fmt.Errorf("checkin: save: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.CheckIn{}, domain.ErrCheckInNotFound
	}
	return toDomain(m), nil
}

func (r *Repository) Delete(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	res := db.Conn(ctx, r.db).
		Where("user_id = ? AND task_id = ? AND date = ?", userID, taskID, date.Time()).
		Delete(&checkInModel{})
	if res.Error != nil {
		return fmt.Errorf("checkin: delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrCheckInNotFound
	}
	return nil
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, f domain.Filter) ([]domain.CheckIn, error) {
	q := db.Conn(ctx, r.db).Where("user_id = ?", userID)
	if f.TaskID != uuid.Nil {
		q = q.Where("task_id = ?", f.TaskID)
	}
	if !f.From.IsZero() {
		q = q.Where("date >= ?", f.From.Time())
	}
	if !f.To.IsZero() {
		q = q.Where("date <= ?", f.To.Time())
	}

	var models []checkInModel
	if err := q.Order("date, task_id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("checkin: list: %w", err)
	}
	checkIns := make([]domain.CheckIn, 0, len(models))
	for _, m := range models {
		checkIns = append(checkIns, toDomain(m))
	}
	return checkIns, nil
}
