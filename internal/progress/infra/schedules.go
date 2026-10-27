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

type scheduleModel struct {
	TaskID    uuid.UUID `gorm:"primaryKey"`
	UntilWeek time.Time `gorm:"primaryKey"`
	UserID    uuid.UUID
	Weekdays  calendar.Weekdays
	CreatedAt time.Time
}

func (scheduleModel) TableName() string {
	return "habit_schedules"
}

func (r *Repository) SaveSchedule(ctx context.Context, userID, taskID uuid.UUID, s domain.PastSchedule) error {
	m := scheduleModel{TaskID: taskID, UntilWeek: s.Until.Time(), UserID: userID, Weekdays: s.Weekdays}
	err := db.Conn(ctx, r.db).Clauses(clause.OnConflict{DoNothing: true}).Create(&m).Error
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return domain.ErrTaskNotFound
	}
	if err != nil {
		return fmt.Errorf("progress: save schedule: %w", err)
	}
	return nil
}

func (r *Repository) Schedules(ctx context.Context, userID uuid.UUID) (map[uuid.UUID][]domain.PastSchedule, error) {
	var models []scheduleModel
	err := db.Conn(ctx, r.db).Where("user_id = ?", userID).Order("task_id, until_week").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("progress: schedules: %w", err)
	}
	schedules := map[uuid.UUID][]domain.PastSchedule{}
	for _, m := range models {
		schedules[m.TaskID] = append(schedules[m.TaskID], domain.PastSchedule{
			Until:    calendar.NewDate(m.UntilWeek.Date()),
			Weekdays: m.Weekdays,
		})
	}
	return schedules, nil
}
