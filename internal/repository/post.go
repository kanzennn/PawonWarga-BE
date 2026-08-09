package repository

import (
	"context"
	"fmt"
	"time"

	"PawonWarga-BE/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// trendFallbackDays is CombinedTrend's lookback window when the caller
// gives no date range at all — see its doc comment.
const trendFallbackDays = 14

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
	// SortBy is one of "newest" (default), "oldest", "engagement" — see
	// postOrderClause. Anything else falls back to "newest".
	SortBy string
}

// postOrderClause maps a PostFilter.SortBy value to a safe, whitelisted SQL
// ORDER BY fragment — never interpolate SortBy into SQL directly, since it
// comes straight from a query param.
func postOrderClause(sortBy string) string {
	switch sortBy {
	case "oldest":
		return "published_at ASC"
	case "engagement":
		return "(like_count + comment_count + share_count) DESC"
	default: // "newest" or unrecognized
		return "published_at DESC"
	}
}

// PostAggregate summarizes a filtered set of posts — used by the /mentions
// dashboard endpoint to report metrics over the whole filtered set, not just
// the current page.
type PostAggregate struct {
	Total           int64
	TotalEngagement int64
	UniqueAuthors   int64
	FlaggedIssues   int64 // count of negative-sentiment posts
	Positive        int64
	Neutral         int64
}

// DailySentimentRow is one day's sentiment split — backs /dashboard/overview's
// trend chart.
type DailySentimentRow struct {
	Day      time.Time
	Positive int64
	Neutral  int64
	Negative int64
}

// PostContentRow is one labeled post's content plus the fields needed to
// aggregate by topic (see AllContent).
type PostContentRow struct {
	Content    string
	Sentiment  model.Sentiment
	Engagement int64
}

// PlatformVolumeRow is one platform's all-time (unfiltered) engagement volume
// and sentiment split — backs the /mentions platform_volume breakdown.
type PlatformVolumeRow struct {
	Platform model.Platform
	Value    int64
	Positive int64
	Negative int64
	Neutral  int64
	Total    int64
}

type PostRepository interface {
	// Upsert inserts a post or, if (platform, platform_post_id) already exists,
	// refreshes the fields that change on re-crawl (engagement counters, content).
	// It never overwrites sentiment fields — those are only touched by UpdateSentiment.
	Upsert(ctx context.Context, post *model.Post) error
	FindByID(ctx context.Context, id uint) (*model.Post, error)
	// List returns sentiment-labeled posts only (sentiment IS NULL means the
	// nightly labeling worker hasn't processed it yet — not dashboard-ready).
	List(ctx context.Context, filter PostFilter) ([]model.Post, int64, error)
	// Aggregate summarizes every labeled post matching filter (ignores
	// filter.Page/PerPage — this covers the whole filtered set).
	Aggregate(ctx context.Context, filter PostFilter) (PostAggregate, error)
	// CountAll returns the total number of labeled posts, ignoring every filter.
	CountAll(ctx context.Context) (int64, error)
	// PlatformVolume returns one row per platform, aggregated over all labeled
	// posts matching the optional date range (from/to nil = all-time) — the
	// dashboard's "big picture" per-platform stats.
	PlatformVolume(ctx context.Context, from, to *time.Time) ([]PlatformVolumeRow, error)
	// CombinedPlatformMentionCounts returns per-platform post+comment counts
	// matching filter (empty filter = everything) — fills
	// platform_volume[].mention_count.
	CombinedPlatformMentionCounts(ctx context.Context, filter PostFilter) (map[model.Platform]int64, error)
	// CombinedAggregate/CombinedTrend/CombinedPlatformVolume/CombinedContent
	// (below) are the /dashboard/overview and /sentiment/overview equivalent
	// of Aggregate/PlatformVolume, but UNION in the comments table too, and
	// CombinedContent is their content+sentiment feed for keyword extraction
	// (service.ExtractTopKeywords/ExtractKeywordStats) and topic
	// classification (service.ClassifyTopics). /mentions must stay
	// post-only (its Mention type maps 1:1 to Post per
	// pawonwarga-fe/docs/api-contract.md), so the existing methods above
	// can't just be changed in place — these are separate queries.
	//
	// Each takes an optional date range (from/to nil = all-time, matching
	// the "view without a date range" option in the frontend's topbar
	// picker) and an optional platform filter (""= all platforms, matching
	// the topbar's platform picker). CombinedTrend additionally falls back
	// to a fixed lookback window when from/to are both nil — see its doc
	// comment. CombinedPlatformVolume deliberately has no platform param —
	// it backs per-platform breakdown panels (Dashboard's Data Sources,
	// Sentiment's "by Platform" chart), which stay unfiltered by platform
	// regardless of the topbar selection, same as /mentions' own
	// PlatformVolume.
	CombinedAggregate(ctx context.Context, from, to *time.Time, platform string) (PostAggregate, error)
	CombinedTrend(ctx context.Context, from, to *time.Time, platform string) ([]DailySentimentRow, error)
	CombinedPlatformVolume(ctx context.Context, from, to *time.Time) ([]PlatformVolumeRow, error)
	CombinedContent(ctx context.Context, from, to *time.Time, platform string) ([]PostContentRow, error)
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

// dateRangeClause returns a SQL fragment ("" if both nil) and its args for
// appending to a raw WHERE clause, e.g. dateRangeClause("published_at", from, to)
// -> (" AND published_at >= ? AND published_at <= ?", []any{from, to}).
// column lets callers disambiguate in joined queries (e.g. "c.published_at").
func dateRangeClause(column string, from, to *time.Time) (string, []any) {
	var clause string
	var args []any
	if from != nil {
		clause += fmt.Sprintf(" AND %s >= ?", column)
		args = append(args, *from)
	}
	if to != nil {
		clause += fmt.Sprintf(" AND %s <= ?", column)
		args = append(args, *to)
	}
	return clause, args
}

// applyFilter applies every PostFilter condition except pagination, plus the
// "labeled only" constraint shared by List/Aggregate/PlatformMentionCounts.
func applyFilter(query *gorm.DB, filter PostFilter) *gorm.DB {
	query = query.Where("sentiment IS NOT NULL")

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
	return query
}

func (r *postRepository) List(ctx context.Context, filter PostFilter) ([]model.Post, int64, error) {
	query := applyFilter(r.db.WithContext(ctx).Model(&model.Post{}), filter)

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
		Order(postOrderClause(filter.SortBy)).
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&posts).Error

	return posts, total, err
}

func (r *postRepository) Aggregate(ctx context.Context, filter PostFilter) (PostAggregate, error) {
	query := applyFilter(r.db.WithContext(ctx).Model(&model.Post{}), filter)

	var agg PostAggregate
	err := query.Select(
		"COUNT(*) AS total",
		"COALESCE(SUM(like_count + comment_count + share_count), 0) AS total_engagement",
		"COUNT(DISTINCT author_handle) AS unique_authors",
		"COALESCE(SUM(CASE WHEN sentiment = 'negative' THEN 1 ELSE 0 END), 0) AS flagged_issues",
		"COALESCE(SUM(CASE WHEN sentiment = 'positive' THEN 1 ELSE 0 END), 0) AS positive",
		"COALESCE(SUM(CASE WHEN sentiment = 'neutral' THEN 1 ELSE 0 END), 0) AS neutral",
	).Scan(&agg).Error
	return agg, err
}

func (r *postRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Post{}).Where("sentiment IS NOT NULL").Count(&count).Error
	return count, err
}

// TODO(caching): when from/to are both nil, this is unfiltered/all-time —
// the result only changes when new data is ingested, not per-request.
// pkg/cache (Redis) is already wired into Router but currently unused
// anywhere in the app; a short TTL (1-5 min) here would cut a full-table
// aggregate query on every /mentions call for the common "no date range"
// case.
func (r *postRepository) PlatformVolume(ctx context.Context, from, to *time.Time) ([]PlatformVolumeRow, error) {
	query := r.db.WithContext(ctx).Model(&model.Post{}).
		Select(
			"platform",
			"COALESCE(SUM(like_count + comment_count + share_count), 0) AS value",
			"COUNT(*) FILTER (WHERE sentiment = 'positive') AS positive",
			"COUNT(*) FILTER (WHERE sentiment = 'negative') AS negative",
			"COUNT(*) FILTER (WHERE sentiment = 'neutral') AS neutral",
			"COUNT(*) AS total",
		).
		Where("sentiment IS NOT NULL")
	if from != nil {
		query = query.Where("published_at >= ?", *from)
	}
	if to != nil {
		query = query.Where("published_at <= ?", *to)
	}

	var rows []PlatformVolumeRow
	err := query.Group("platform").Scan(&rows).Error
	return rows, err
}

func (r *postRepository) CombinedPlatformMentionCounts(ctx context.Context, filter PostFilter) (map[model.Platform]int64, error) {
	postWhere := "sentiment IS NOT NULL"
	commentWhere := "c.sentiment IS NOT NULL"
	var postArgs, commentArgs []any

	if filter.Platform != "" {
		postWhere += " AND platform = ?"
		postArgs = append(postArgs, filter.Platform)
		commentWhere += " AND p.platform = ?"
		commentArgs = append(commentArgs, filter.Platform)
	}
	if filter.Sentiment != "" {
		postWhere += " AND sentiment = ?"
		postArgs = append(postArgs, filter.Sentiment)
		commentWhere += " AND c.sentiment = ?"
		commentArgs = append(commentArgs, filter.Sentiment)
	}
	if filter.Query != "" {
		like := "%" + filter.Query + "%"
		postWhere += " AND (content ILIKE ? OR author_handle ILIKE ?)"
		postArgs = append(postArgs, like, like)
		commentWhere += " AND (c.content ILIKE ? OR c.author_handle ILIKE ?)"
		commentArgs = append(commentArgs, like, like)
	}
	postDateClause, postDateArgs := dateRangeClause("published_at", filter.From, filter.To)
	postWhere += postDateClause
	postArgs = append(postArgs, postDateArgs...)
	commentDateClause, commentDateArgs := dateRangeClause("c.published_at", filter.From, filter.To)
	commentWhere += commentDateClause
	commentArgs = append(commentArgs, commentDateArgs...)

	sql := fmt.Sprintf(`
		WITH combined AS (
			SELECT platform FROM posts WHERE %s
			UNION ALL
			SELECT p.platform FROM comments c JOIN posts p ON p.id = c.post_id WHERE %s
		)
		SELECT platform, COUNT(*) AS count FROM combined GROUP BY platform
	`, postWhere, commentWhere)

	args := append(postArgs, commentArgs...)

	var rows []struct {
		Platform model.Platform
		Count    int64
	}
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[model.Platform]int64, len(rows))
	for _, row := range rows {
		counts[row.Platform] = row.Count
	}
	return counts, nil
}

// TODO(caching): when from/to are both nil, this is unfiltered/all-time —
// every Combined* query below scans both posts and comments in full, the
// most expensive queries in this file.
func (r *postRepository) CombinedAggregate(ctx context.Context, from, to *time.Time, platform string) (PostAggregate, error) {
	postClause, postArgs := dateRangeClause("published_at", from, to)
	commentClause, commentArgs := dateRangeClause("c.published_at", from, to)
	if platform != "" {
		postClause += " AND platform = ?"
		postArgs = append(postArgs, platform)
		commentClause += " AND p.platform = ?"
		commentArgs = append(commentArgs, platform)
	}

	sql := fmt.Sprintf(`
		WITH combined AS (
			SELECT sentiment, author_handle, (like_count + comment_count + share_count) AS engagement
			FROM posts WHERE sentiment IS NOT NULL%s
			UNION ALL
			SELECT c.sentiment, c.author_handle, c.like_count AS engagement
			FROM comments c JOIN posts p ON p.id = c.post_id
			WHERE c.sentiment IS NOT NULL%s
		)
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(engagement), 0) AS total_engagement,
			COUNT(DISTINCT author_handle) AS unique_authors,
			COALESCE(SUM(CASE WHEN sentiment = 'negative' THEN 1 ELSE 0 END), 0) AS flagged_issues,
			COALESCE(SUM(CASE WHEN sentiment = 'positive' THEN 1 ELSE 0 END), 0) AS positive,
			COALESCE(SUM(CASE WHEN sentiment = 'neutral' THEN 1 ELSE 0 END), 0) AS neutral
		FROM combined
	`, postClause, commentClause)

	var agg PostAggregate
	err := r.db.WithContext(ctx).Raw(sql, append(postArgs, commentArgs...)...).Scan(&agg).Error
	return agg, err
}

// CombinedTrend falls back to a fixed lookback window only when the caller
// gives no date range at all (from and to both nil) — matching the "no
// date range" / all-time default elsewhere. An explicit range (even a
// single-sided one) is used exactly as given, however long or short.
func (r *postRepository) CombinedTrend(ctx context.Context, from, to *time.Time, platform string) ([]DailySentimentRow, error) {
	if from == nil && to == nil {
		since := time.Now().AddDate(0, 0, -trendFallbackDays)
		from = &since
	}

	postClause, postArgs := dateRangeClause("published_at", from, to)
	commentClause, commentArgs := dateRangeClause("c.published_at", from, to)
	if platform != "" {
		postClause += " AND platform = ?"
		postArgs = append(postArgs, platform)
		commentClause += " AND p.platform = ?"
		commentArgs = append(commentArgs, platform)
	}

	sql := fmt.Sprintf(`
		WITH combined AS (
			SELECT published_at, sentiment FROM posts
			WHERE sentiment IS NOT NULL%s
			UNION ALL
			SELECT c.published_at, c.sentiment
			FROM comments c JOIN posts p ON p.id = c.post_id
			WHERE c.sentiment IS NOT NULL%s
		)
		SELECT
			DATE(published_at) AS day,
			COUNT(*) FILTER (WHERE sentiment = 'positive') AS positive,
			COUNT(*) FILTER (WHERE sentiment = 'neutral') AS neutral,
			COUNT(*) FILTER (WHERE sentiment = 'negative') AS negative
		FROM combined
		GROUP BY DATE(published_at)
		ORDER BY DATE(published_at) ASC
	`, postClause, commentClause)

	var rows []DailySentimentRow
	err := r.db.WithContext(ctx).Raw(sql, append(postArgs, commentArgs...)...).Scan(&rows).Error
	return rows, err
}

func (r *postRepository) CombinedPlatformVolume(ctx context.Context, from, to *time.Time) ([]PlatformVolumeRow, error) {
	postClause, postArgs := dateRangeClause("published_at", from, to)
	commentClause, commentArgs := dateRangeClause("c.published_at", from, to)

	sql := fmt.Sprintf(`
		WITH combined AS (
			SELECT platform, sentiment, (like_count + comment_count + share_count) AS engagement
			FROM posts WHERE sentiment IS NOT NULL%s
			UNION ALL
			SELECT p.platform, c.sentiment, c.like_count AS engagement
			FROM comments c
			JOIN posts p ON p.id = c.post_id
			WHERE c.sentiment IS NOT NULL%s
		)
		SELECT
			platform,
			COALESCE(SUM(engagement), 0) AS value,
			COUNT(*) FILTER (WHERE sentiment = 'positive') AS positive,
			COUNT(*) FILTER (WHERE sentiment = 'negative') AS negative,
			COUNT(*) FILTER (WHERE sentiment = 'neutral') AS neutral,
			COUNT(*) AS total
		FROM combined
		GROUP BY platform
	`, postClause, commentClause)

	var rows []PlatformVolumeRow
	err := r.db.WithContext(ctx).Raw(sql, append(postArgs, commentArgs...)...).Scan(&rows).Error
	return rows, err
}

func (r *postRepository) CombinedContent(ctx context.Context, from, to *time.Time, platform string) ([]PostContentRow, error) {
	postClause, postArgs := dateRangeClause("published_at", from, to)
	commentClause, commentArgs := dateRangeClause("c.published_at", from, to)
	if platform != "" {
		postClause += " AND platform = ?"
		postArgs = append(postArgs, platform)
		commentClause += " AND p.platform = ?"
		commentArgs = append(commentArgs, platform)
	}

	sql := fmt.Sprintf(`
		SELECT content, sentiment, (like_count + comment_count + share_count) AS engagement
		FROM posts WHERE sentiment IS NOT NULL%s
		UNION ALL
		SELECT c.content, c.sentiment, c.like_count AS engagement
		FROM comments c JOIN posts p ON p.id = c.post_id
		WHERE c.sentiment IS NOT NULL%s
	`, postClause, commentClause)

	var rows []PostContentRow
	err := r.db.WithContext(ctx).Raw(sql, append(postArgs, commentArgs...)...).Scan(&rows).Error
	return rows, err
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
