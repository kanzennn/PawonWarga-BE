package repository

import (
	"context"
	"time"

	"PawonWarga-BE/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PostFilter drives the query params the dashboard filters on
// (platform, sentiment, category, free-text search, date range) plus pagination.
type PostFilter struct {
	Platform  string
	Sentiment string
	Category  string
	Query     string
	From      *time.Time
	To        *time.Time
	Page      int
	PerPage   int
}

type PostRepository interface {
	// Upsert inserts a post or, if (platform, platform_post_id) already exists,
	// refreshes the fields that change on re-crawl (engagement counters, content).
	// It never overwrites sentiment fields — those are only touched by UpdateSentiment.
	Upsert(ctx context.Context, post *model.Post) error
	FindByID(ctx context.Context, id uint) (*model.Post, error)
	List(ctx context.Context, filter PostFilter) ([]model.Post, int64, error)
	// FindUnlabeled returns posts where sentiment IS NULL, oldest first — used by the
	// nightly labeling worker.
	FindUnlabeled(ctx context.Context, limit int) ([]model.Post, error)
	UpdateSentiment(ctx context.Context, id uint, sentiment model.Sentiment, score float32, modelVersion string) error
}

type postRepository struct {
	db *gorm.DB
}

func NewPostRepository(db *gorm.DB) PostRepository {
	return &postRepository{db: db}
}

func (r *postRepository) Upsert(ctx context.Context, post *model.Post) error {
	// Columns must exactly match the (platform, platform_post_id, published_at)
	// unique index — that's the real dedup key now that published_at is part of
	// it (required by TimescaleDB for hypertable unique constraints).
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "platform"}, {Name: "platform_post_id"}, {Name: "published_at"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"content", "author_handle", "author_name", "url", "location",
				"like_count", "comment_count", "share_count", "view_count",
				"crawled_at", "raw_payload",
			}),
		}).
		Create(post).Error
}

func (r *postRepository) FindByID(ctx context.Context, id uint) (*model.Post, error) {
	// id is globally unique on its own (serial), but Post has a composite
	// primary key (id, published_at) for TimescaleDB — so look up with an
	// explicit WHERE instead of GORM's First(&dest, id) shortcut, which only
	// handles single-column primary keys reliably.
	var post model.Post
	if err := r.db.WithContext(ctx).Preload("Comments").Where("id = ?", id).First(&post).Error; err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *postRepository) List(ctx context.Context, filter PostFilter) ([]model.Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Post{})

	if filter.Platform != "" {
		query = query.Where("platform = ?", filter.Platform)
	}
	if filter.Sentiment != "" {
		query = query.Where("sentiment = ?", filter.Sentiment)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		query = query.Where("content ILIKE ? OR author_handle ILIKE ?", like, like)
	}
	if filter.From != nil {
		query = query.Where("published_at >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("published_at <= ?", *filter.To)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, perPage := filter.Page, filter.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	var posts []model.Post
	err := query.
		Order("published_at DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&posts).Error

	return posts, total, err
}

func (r *postRepository) FindUnlabeled(ctx context.Context, limit int) ([]model.Post, error) {
	var posts []model.Post
	err := r.db.WithContext(ctx).
		Where("sentiment IS NULL").
		Order("published_at ASC").
		Limit(limit).
		Find(&posts).Error
	return posts, err
}

func (r *postRepository) UpdateSentiment(ctx context.Context, id uint, sentiment model.Sentiment, score float32, modelVersion string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.Post{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"sentiment":       sentiment,
			"sentiment_score": score,
			"model_version":   modelVersion,
			"labeled_at":      now,
		}).Error
}
