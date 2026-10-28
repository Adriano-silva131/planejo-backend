package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db"
	"github.com/Adriano-silva131/planejo-backend/internal/task/domain"
)

type areaColorModel struct {
	UserID    uuid.UUID `gorm:"primaryKey"`
	Area      string    `gorm:"primaryKey"`
	Color     string
	UpdatedAt time.Time
}

func (areaColorModel) TableName() string {
	return "area_colors"
}

func (r *Repository) Areas(ctx context.Context, userID uuid.UUID) ([]domain.AreaUsage, error) {
	var areas []domain.AreaUsage
	err := db.Conn(ctx, r.db).Model(&taskModel{}).
		Select("tasks.area, count(*) AS tasks, coalesce(max(area_colors.color), '') AS color").
		Joins("LEFT JOIN area_colors ON area_colors.user_id = tasks.user_id AND area_colors.area = tasks.area").
		Where("tasks.user_id = ?", userID).
		Group("tasks.area").
		Order("tasks DESC, tasks.area").
		Scan(&areas).Error
	if err != nil {
		return nil, fmt.Errorf("task: areas: %w", err)
	}
	return areas, nil
}

func (r *Repository) SetAreaColor(ctx context.Context, userID uuid.UUID, c domain.AreaColor) error {
	m := areaColorModel{UserID: userID, Area: c.Area, Color: c.Color}
	err := db.Conn(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "area"}},
		DoUpdates: clause.AssignmentColumns([]string{"color", "updated_at"}),
	}).Create(&m).Error
	if err != nil {
		return fmt.Errorf("task: set area color: %w", err)
	}
	return nil
}
