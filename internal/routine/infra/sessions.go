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

// sessionModel is the routine_sessions row: a time, from start_minute to
// end_minute (1440 is midnight), or only a period.
type sessionModel struct {
	TaskID      uuid.UUID `gorm:"primaryKey"`
	Date        time.Time `gorm:"primaryKey"`
	UserID      uuid.UUID
	StartMinute *int
	EndMinute   *int
	Period      *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (sessionModel) TableName() string {
	return "routine_sessions"
}

func toSessionModel(s domain.Session) sessionModel {
	m := sessionModel{
		TaskID: s.TaskID, Date: s.Date.Time(), UserID: s.UserID, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
	if !s.Timed() {
		period := string(s.Period)
		m.Period = &period
		return m
	}
	start, end := int(s.Start), int(s.End)
	m.StartMinute, m.EndMinute = &start, &end
	return m
}

func toSessionDomain(m sessionModel) domain.Session {
	s := domain.Session{
		UserID:    m.UserID,
		TaskID:    m.TaskID,
		Date:      calendar.NewDate(m.Date.Date()),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
	if m.Period != nil {
		s.Period = calendar.Period(*m.Period)
		return s
	}
	if m.StartMinute != nil && m.EndMinute != nil {
		s.Start, s.End = calendar.TimeOfDay(*m.StartMinute), calendar.TimeOfDay(*m.EndMinute)
	}
	return s
}

type SessionRepository struct {
	db *gorm.DB
}

var _ domain.Sessions = (*SessionRepository)(nil)

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

var sessionUpsert = db.OwnedUpsert{
	Table:    "routine_sessions",
	Conflict: []string{"task_id", "date"},
	Update:   []string{"start_minute", "end_minute", "period", "updated_at"},
}

func (r *SessionRepository) Save(ctx context.Context, s domain.Session) (domain.Session, error) {
	m := toSessionModel(s)
	err := sessionUpsert.Save(ctx, r.db, &m, m.UserID)
	switch {
	case errors.Is(err, db.ErrNotOwned), errors.Is(err, gorm.ErrForeignKeyViolated):
		return domain.Session{}, domain.ErrTaskNotFound
	case err != nil:
		return domain.Session{}, fmt.Errorf("routine: save session: %w", err)
	}
	return toSessionDomain(m), nil
}

func (r *SessionRepository) Delete(ctx context.Context, userID, taskID uuid.UUID, date calendar.Date) error {
	key := db.TaskDay{UserID: userID, TaskID: taskID, Date: date.Time()}
	deleted, err := db.DeleteTaskDay(ctx, r.db, &sessionModel{}, key)
	switch {
	case err != nil:
		return fmt.Errorf("routine: delete session: %w", err)
	case !deleted:
		return domain.ErrSessionNotFound
	}
	return nil
}

func (r *SessionRepository) List(ctx context.Context, userID uuid.UUID, from, to calendar.Date) ([]domain.Session, error) {
	var models []sessionModel
	err := inPeriod(db.Conn(ctx, r.db), userID, from, to).
		Order("date, start_minute, task_id").
		Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("routine: list sessions: %w", err)
	}
	sessions := make([]domain.Session, 0, len(models))
	for _, m := range models {
		sessions = append(sessions, toSessionDomain(m))
	}
	return sessions, nil
}

func (r *SessionRepository) DeleteForTask(ctx context.Context, userID, taskID uuid.UUID) error {
	err := db.Conn(ctx, r.db).Where("user_id = ? AND task_id = ?", userID, taskID).Delete(&sessionModel{}).Error
	if err != nil {
		return fmt.Errorf("routine: delete sessions of task: %w", err)
	}
	return nil
}
