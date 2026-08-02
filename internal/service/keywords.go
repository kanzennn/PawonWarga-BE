package service

import (
	"context"
	"time"

	"PawonWarga-BE/internal/repository"
)

// trackedKeywordsLimit is how many distinct keywords /keywords tracks —
// the handler's q/sentiment filters narrow this list further, they don't
// change what gets extracted in the first place.
const trackedKeywordsLimit = 50

type KeywordsOverview struct {
	Keywords []KeywordStat
}

type KeywordService interface {
	GetKeywords(ctx context.Context, from, to *time.Time) (KeywordsOverview, error)
}

type keywordService struct {
	postRepo repository.PostRepository
}

func NewKeywordService(postRepo repository.PostRepository) KeywordService {
	return &keywordService{postRepo: postRepo}
}

// GetKeywords: from/to are nil when the topbar date-range picker is
// cleared ("view without a date range") — extraction then covers all-time.
func (s *keywordService) GetKeywords(ctx context.Context, from, to *time.Time) (KeywordsOverview, error) {
	contentRows, err := s.postRepo.CombinedContent(ctx, from, to)
	if err != nil {
		return KeywordsOverview{}, err
	}

	return KeywordsOverview{
		Keywords: ExtractKeywordStats(contentRows, trackedKeywordsLimit),
	}, nil
}
