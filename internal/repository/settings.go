package repository

import (
	"context"

	"PawonWarga-BE/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingsRepository interface {
	ListUserPreferences(ctx context.Context, userID uint) ([]model.UserPreference, error)
	// UpsertUserPreference upserts by (user_id, key).
	UpsertUserPreference(ctx context.Context, userID uint, key string, checked bool) error
}

type settingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) SettingsRepository {
	return &settingsRepository{db: db}
}

func (r *settingsRepository) ListUserPreferences(ctx context.Context, userID uint) ([]model.UserPreference, error) {
	var rows []model.UserPreference
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error
	return rows, err
}

func (r *settingsRepository) UpsertUserPreference(ctx context.Context, userID uint, key string, checked bool) error {
	row := model.UserPreference{UserID: userID, Key: key, Checked: checked}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"checked", "updated_at"}),
		}).
		Create(&row).Error
}
