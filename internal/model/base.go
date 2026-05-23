package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel replaces gorm.Model with explicit JSON tags so all tables
// consistently expose id, created_at, and updated_at in API responses.
// deleted_at is kept for soft-delete support but hidden from responses.
type BaseModel struct {
	ID        uint           `gorm:"primarykey"  json:"id"`
	CreatedAt time.Time      `                   json:"created_at"`
	UpdatedAt time.Time      `                   json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"       json:"-"`
}
