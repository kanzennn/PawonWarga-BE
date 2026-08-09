package service

import (
	"context"
	"time"

	"PawonWarga-BE/internal/repository"
)

// SentimentOverview bundles every aggregate /sentiment/overview needs. Like
// /dashboard/overview, everything here combines posts AND comments (see
// repository.PostRepository's Combined* methods) — /mentions is the only
// endpoint that stays post-only.
type SentimentOverview struct {
	Total          int64
	Positive       int64
	Negative       int64
	Neutral        int64
	Trend          []repository.DailySentimentRow
	PlatformVolume []repository.PlatformVolumeRow
	ByCategory     []CategorySentimentStat
}

type SentimentService interface {
	GetOverview(ctx context.Context, from, to *time.Time, platform string) (SentimentOverview, error)
}

type sentimentService struct {
	postRepo repository.PostRepository
}

func NewSentimentService(postRepo repository.PostRepository) SentimentService {
	return &sentimentService{postRepo: postRepo}
}

// GetOverview: from/to are nil when the topbar date-range picker is cleared
// ("view without a date range") — every aggregate then covers all-time
// (CombinedTrend still applies its own fallback lookback window in that
// case — see its doc comment). platform is "" when the topbar's platform
// picker is cleared ("All Platforms"). PlatformVolume (backing the
// "Sentiment by Platform" chart) deliberately ignores platform — see
// CombinedPlatformVolume's doc comment on the repository interface.
func (s *sentimentService) GetOverview(ctx context.Context, from, to *time.Time, platform string) (SentimentOverview, error) {
	agg, err := s.postRepo.CombinedAggregate(ctx, from, to, platform)
	if err != nil {
		return SentimentOverview{}, err
	}

	trend, err := s.postRepo.CombinedTrend(ctx, from, to, platform)
	if err != nil {
		return SentimentOverview{}, err
	}

	platformVolume, err := s.postRepo.CombinedPlatformVolume(ctx, from, to)
	if err != nil {
		return SentimentOverview{}, err
	}

	contentRows, err := s.postRepo.CombinedContent(ctx, from, to, platform)
	if err != nil {
		return SentimentOverview{}, err
	}
	byCategory := ClassifyTopicSentiment(contentRows)

	return SentimentOverview{
		Total:          agg.Total,
		Positive:       agg.Positive,
		Negative:       agg.FlaggedIssues,
		Neutral:        agg.Neutral,
		Trend:          trend,
		PlatformVolume: platformVolume,
		ByCategory:     byCategory,
	}, nil
}
