package repository

import (
	"context"
	"time"

	"PawonWarga-BE/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommentFilter mirrors the PostFilter dimensions that make sense for
// comments — used to keep /mentions' "total_comments" metric consistent
// with whatever platform/sentiment/query filter is currently applied to
// the post list. Platform requires a join since Comment has no platform
// column of its own (see CountFiltered).
type CommentFilter struct {
	Platform  string
	Sentiment string
	Query     string
	From      *time.Time
	To        *time.Time
}

type CommentRepository interface {
	// Upsert inserts a comment or, if (post_id, platform_comment_id) already exists,
	// refreshes fields that change on re-crawl. Never touches sentiment fields.
	Upsert(ctx context.Context, comment *model.Comment) error
	// ListByPostID returns a page of labeled comments on postID (oldest
	// first) plus the total count matching (for pagination) — backs
	// GET /mentions/{id}/comments.
	ListByPostID(ctx context.Context, postID uint, page, perPage int) ([]model.Comment, int64, error)
	// CountFiltered returns the number of labeled comments matching filter,
	// joined to their parent post for the platform dimension.
	CountFiltered(ctx context.Context, filter CommentFilter) (int64, error)
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

func (r *commentRepository) ListByPostID(ctx context.Context, postID uint, page, perPage int) ([]model.Comment, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Comment{}).
		Where("post_id = ? AND sentiment IS NOT NULL", postID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	var comments []model.Comment
	err := query.
		Order("published_at ASC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&comments).Error

	return comments, total, err
}

func (r *commentRepository) CountFiltered(ctx context.Context, filter CommentFilter) (int64, error) {
	query := r.db.WithContext(ctx).
		Table("comments AS c").
		Joins("JOIN posts p ON p.id = c.post_id").
		Where("c.sentiment IS NOT NULL")

	if filter.Platform != "" {
		query = query.Where("p.platform = ?", filter.Platform)
	}
	if filter.Sentiment != "" {
		query = query.Where("c.sentiment = ?", filter.Sentiment)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		query = query.Where("c.content ILIKE ? OR c.author_handle ILIKE ?", like, like)
	}
	if filter.From != nil {
		query = query.Where("c.published_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("c.published_at <= ?", *filter.To)
	}

	var count int64
	err := query.Count(&count).Error
	return count, err
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
