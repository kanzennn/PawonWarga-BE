package repository

import (
	"context"
	"time"

	"PawonWarga-BE/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommentRepository interface {
	// Upsert inserts a comment or, if (post_id, platform_comment_id) already exists,
	// refreshes fields that change on re-crawl. Never touches sentiment fields.
	Upsert(ctx context.Context, comment *model.Comment) error
	ListByPostID(ctx context.Context, postID uint) ([]model.Comment, error)
	FindUnlabeled(ctx context.Context, limit int) ([]model.Comment, error)
	UpdateSentiment(ctx context.Context, id uint, sentiment model.Sentiment, score float32, modelVersion string) error
}

type commentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

func (r *commentRepository) Upsert(ctx context.Context, comment *model.Comment) error {
	// Columns must exactly match the (post_id, platform_comment_id, published_at)
	// unique index — published_at is part of it because TimescaleDB requires the
	// hypertable's partitioning column in every unique constraint.
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "post_id"}, {Name: "platform_comment_id"}, {Name: "published_at"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"content", "author_handle", "like_count", "crawled_at", "raw_payload",
			}),
		}).
		Create(comment).Error
}

func (r *commentRepository) ListByPostID(ctx context.Context, postID uint) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Order("published_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *commentRepository) FindUnlabeled(ctx context.Context, limit int) ([]model.Comment, error) {
	var comments []model.Comment
	err := r.db.WithContext(ctx).
		Where("sentiment IS NULL").
		Order("published_at ASC").
		Limit(limit).
		Find(&comments).Error
	return comments, err
}

func (r *commentRepository) UpdateSentiment(ctx context.Context, id uint, sentiment model.Sentiment, score float32, modelVersion string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Comment{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"sentiment":       sentiment,
			"sentiment_score": score,
			"model_version":   modelVersion,
			"labeled_at":      now,
		}).Error
}
