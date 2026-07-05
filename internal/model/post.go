package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type Platform string

const (
	PlatformX         Platform = "x"
	PlatformInstagram Platform = "instagram"
	PlatformTikTok    Platform = "tiktok"
	PlatformYouTube   Platform = "youtube"
	PlatformNews      Platform = "news"
)

type Sentiment string

const (
	SentimentPositive Sentiment = "positive"
	SentimentNegative Sentiment = "negative"
	SentimentNeutral  Sentiment = "neutral"
)

// Post and Comment (see comment.go) are TimescaleDB hypertables partitioned
// on PublishedAt (see pkg/database.EnsureHypertables). TimescaleDB requires
// every primary key / unique constraint on a hypertable to include the
// partitioning column, so — unlike other models — these do NOT embed
// BaseModel and instead declare a composite primary key of (id, published_at).
//
// TimescaleDB also does not support foreign keys that reference a
// hypertable, so Comment.PostID is a plain indexed column, not a DB-level
// FK constraint (FK constraint creation is disabled globally in
// pkg/database.NewPostgres). GORM's Preload still works for reads — it only
// needs the foreignKey tag, not an actual constraint. This means deleting a
// Post does NOT cascade-delete its Comments at the DB level; that must be
// handled in application code if a hard delete is ever needed.

// Post is a crawled post/caption/article, one row per (platform, platform_post_id, published_at).
// Sentiment fields are nullable — NULL means the nightly labeling worker
// hasn't processed it yet (see repository.FindUnlabeled).
type Post struct {
	ID             uint            `gorm:"primaryKey;autoIncrement"                                                                                json:"id"`
	Platform       Platform        `gorm:"size:20;not null;uniqueIndex:idx_posts_platform_post;check:platform IN ('x','instagram','tiktok','youtube','news')" json:"platform"`
	PlatformPostID string          `gorm:"size:255;not null;uniqueIndex:idx_posts_platform_post"                                                   json:"platform_post_id"`
	AuthorHandle   *string         `gorm:"size:255"                                                                                                 json:"author_handle"`
	AuthorName     *string         `gorm:"size:255"                                                                                                 json:"author_name"`
	Content        string          `gorm:"type:text;not null"                                                                                       json:"content"`
	URL            *string         `gorm:"size:1000"                                                                                                json:"url"`
	Location       *string         `gorm:"size:255"                                                                                                 json:"location"`
	Category       *string         `gorm:"size:100;index"                                                                                           json:"category"`
	LikeCount      int             `gorm:"not null;default:0"                                                                                       json:"like_count"`
	CommentCount   int             `gorm:"not null;default:0"                                                                                       json:"comment_count"`
	ShareCount     int             `gorm:"not null;default:0"                                                                                       json:"share_count"`
	ViewCount      int             `gorm:"not null;default:0"                                                                                       json:"view_count"`
	Sentiment      *Sentiment      `gorm:"size:10;index;check:sentiment IN ('positive','negative','neutral')"                                       json:"sentiment"`
	SentimentScore *float32        `json:"sentiment_score"`
	ModelVersion   *string         `gorm:"size:100"                                                                                                 json:"model_version"`
	PublishedAt    time.Time       `gorm:"primaryKey;not null;uniqueIndex:idx_posts_platform_post"                                                 json:"published_at"`
	CrawledAt      time.Time       `gorm:"not null"                                                                                                 json:"crawled_at"`
	LabeledAt      *time.Time      `json:"labeled_at"`
	RawPayload     json.RawMessage `gorm:"type:jsonb"                                                                                               json:"-"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      gorm.DeletedAt  `gorm:"index"                                                                                                    json:"-"`
	Comments       []Comment       `gorm:"foreignKey:PostID"                                                                                        json:"comments,omitempty"`
}
