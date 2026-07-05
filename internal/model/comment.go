package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// Comment is a crawled comment on a Post, labeled independently from its
// parent post. See post.go for why it has a composite primary key and no
// DB-level FK to Post.
type Comment struct {
	ID                uint            `gorm:"primaryKey;autoIncrement"                                           json:"id"`
	PostID            uint            `gorm:"not null;index;uniqueIndex:idx_comments_post_comment"               json:"post_id"`
	PlatformCommentID string          `gorm:"size:255;not null;uniqueIndex:idx_comments_post_comment"            json:"platform_comment_id"`
	AuthorHandle      *string         `gorm:"size:255"                                                            json:"author_handle"`
	Content           string          `gorm:"type:text;not null"                                                  json:"content"`
	LikeCount         int             `gorm:"not null;default:0"                                                  json:"like_count"`
	Sentiment         *Sentiment      `gorm:"size:10;index;check:sentiment IN ('positive','negative','neutral')"  json:"sentiment"`
	SentimentScore    *float32        `json:"sentiment_score"`
	ModelVersion      *string         `gorm:"size:100"                                                            json:"model_version"`
	PublishedAt       time.Time       `gorm:"primaryKey;not null;uniqueIndex:idx_comments_post_comment"          json:"published_at"`
	CrawledAt         time.Time       `gorm:"not null"                                                            json:"crawled_at"`
	LabeledAt         *time.Time      `json:"labeled_at"`
	RawPayload        json.RawMessage `gorm:"type:jsonb"                                                          json:"-"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index"                                                               json:"-"`
}
