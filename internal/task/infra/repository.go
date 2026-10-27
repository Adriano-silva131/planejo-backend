package infra

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

type Repository struct {
	db *gorm.DB
}

var _ domain.Repository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, t domain.Task) (domain.Task, error) {
	m := toModel(t)
	if err := db.Conn(ctx, r.db).Clauses(clause.Returning{}).Create(&m).Error; err != nil {
		return domain.Task{}, fmt.Errorf("task: create: %w", err)
	}
	return toDomain(m)
}

func (r *Repository) Get(ctx context.Context, userID, id uuid.UUID) (domain.Task, error) {
	var m taskModel
	err := db.Conn(ctx, r.db).Where("id = ? AND user_id = ?", id, userID).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("task: get: %w", err)
	}
	return toDomain(m)
}

func (r *Repository) List(ctx context.Context, userID uuid.UUID, offset, limit int) ([]domain.Task, int, error) {
	q := db.Conn(ctx, r.db).Model(&taskModel{}).Where("user_id = ?", userID).Session(&gorm.Session{})

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("task: count: %w", err)
	}
	var models []taskModel
	err := q.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&models).Error
	if err != nil {
		return nil, 0, fmt.Errorf("task: list: %w", err)
	}

	tasks, err := toDomainList(models)
	if err != nil {
		return nil, 0, err
	}
	return tasks, int(total), nil
}

func (r *Repository) All(ctx context.Context, userID uuid.UUID) ([]domain.Task, error) {
	var models []taskModel
	err := db.Conn(ctx, r.db).Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("task: all: %w", err)
	}
	return toDomainList(models)
}

func (r *Repository) Areas(ctx context.Context, userID uuid.UUID) ([]domain.AreaUsage, error) {
	var areas []domain.AreaUsage
	err := db.Conn(ctx, r.db).Model(&taskModel{}).
		Select("area, count(*) AS tasks").
		Where("user_id = ?", userID).
		Group("area").
		Order("tasks DESC, area").
		Scan(&areas).Error
	if err != nil {
		return nil, fmt.Errorf("task: areas: %w", err)
	}
	return areas, nil
}

func toDomainList(models []taskModel) ([]domain.Task, error) {
	tasks := make([]domain.Task, 0, len(models))
	for _, m := range models {
		t, err := toDomain(m)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (r *Repository) Update(ctx context.Context, t domain.Task) (domain.Task, error) {
	m := toModel(t)
	res := db.Conn(ctx, r.db).Model(&m).Clauses(clause.Returning{}).
		Where("user_id = ?", t.UserID).
		Select(editableColumns).
		Updates(&m)
	if res.Error != nil {
		return domain.Task{}, fmt.Errorf("task: update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.Task{}, domain.ErrTaskNotFound
	}
	return toDomain(m)
}

func (r *Repository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	res := db.Conn(ctx, r.db).Where("id = ? AND user_id = ?", id, userID).Delete(&taskModel{})
	if res.Error != nil {
		return fmt.Errorf("task: delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return domain.ErrTaskNotFound
	}
	return nil
}
