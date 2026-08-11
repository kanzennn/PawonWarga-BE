package service

import (
	"context"
	"errors"

	"PawonWarga-BE/internal/model"
	"PawonWarga-BE/internal/repository"
)

var ErrUnknownPreference = errors.New("unknown preference key")

// integrationPlatforms fixes the order /settings/integrations reports
// platforms in — same set and order as handler.platformDisplayOrder, kept
// as its own copy here since service can't import handler.
var integrationPlatforms = []model.Platform{
	model.PlatformX,
	model.PlatformInstagram,
	model.PlatformTikTok,
	model.PlatformNews,
	model.PlatformYouTube,
}

// PreferenceDef is one entry in the fixed monitoring-preference catalog —
// title/desc/default live in code, not the database, so copy can change
// without a migration (matches docs/api-contract.md's "come from the
// backend" note on pawonwarga-fe). Keys and copy mirror the frontend's old
// dummy-data.ts exactly, so switching from mock to real data is invisible.
type PreferenceDef struct {
	Key            string
	Title          string
	Desc           string
	DefaultChecked bool
}

var PreferenceCatalog = []PreferenceDef{
	{Key: "negativeAlert", Title: "Negative sentiment alert", Desc: "Notify when negative sentiment rises above threshold.", DefaultChecked: true},
	{Key: "dailySummary", Title: "Daily AI summary", Desc: "Receive daily summary of MBG public conversations.", DefaultChecked: true},
	{Key: "keywordSpike", Title: "Keyword spike detection", Desc: "Detect sudden increase in monitored keywords.", DefaultChecked: true},
	{Key: "weeklyReport", Title: "Weekly report email", Desc: "Send scheduled sentiment report every Monday.", DefaultChecked: false},
}

// PreferenceItem is one resolved preference: catalog copy plus this user's
// stored value, or the catalog default if they've never touched it.
type PreferenceItem struct {
	Key     string
	Title   string
	Desc    string
	Checked bool
}

// IntegrationItem is one platform's connectivity. Both Status and Count are
// computed live from actual ingested data, never persisted — see
// GetIntegrations.
type IntegrationItem struct {
	Platform model.Platform
	Status   model.IntegrationStatus
	Count    int64
}

type SettingsService interface {
	GetPreferences(ctx context.Context, userID uint) ([]PreferenceItem, error)
	UpdatePreference(ctx context.Context, userID uint, key string, checked bool) ([]PreferenceItem, error)

	GetIntegrations(ctx context.Context) ([]IntegrationItem, error)
}

type settingsService struct {
	settingsRepo repository.SettingsRepository
	postRepo     repository.PostRepository
}

func NewSettingsService(settingsRepo repository.SettingsRepository, postRepo repository.PostRepository) SettingsService {
	return &settingsService{settingsRepo: settingsRepo, postRepo: postRepo}
}

func (s *settingsService) GetPreferences(ctx context.Context, userID uint) ([]PreferenceItem, error) {
	overrides, err := s.settingsRepo.ListUserPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	return mergePreferences(overrides), nil
}

func (s *settingsService) UpdatePreference(ctx context.Context, userID uint, key string, checked bool) ([]PreferenceItem, error) {
	if !isKnownPreference(key) {
		return nil, ErrUnknownPreference
	}
	if err := s.settingsRepo.UpsertUserPreference(ctx, userID, key, checked); err != nil {
		return nil, err
	}
	return s.GetPreferences(ctx, userID)
}

func mergePreferences(overrides []model.UserPreference) []PreferenceItem {
	checkedByKey := make(map[string]bool, len(overrides))
	for _, o := range overrides {
		checkedByKey[o.Key] = o.Checked
	}

	items := make([]PreferenceItem, len(PreferenceCatalog))
	for i, def := range PreferenceCatalog {
		checked := def.DefaultChecked
		if v, ok := checkedByKey[def.Key]; ok {
			checked = v
		}
		items[i] = PreferenceItem{Key: def.Key, Title: def.Title, Desc: def.Desc, Checked: checked}
	}
	return items
}

func isKnownPreference(key string) bool {
	for _, def := range PreferenceCatalog {
		if def.Key == key {
			return true
		}
	}
	return false
}

// GetIntegrations reports every platform in integrationPlatforms, all-time
// (no date range — matches GET /settings/integrations taking no query
// params). Status is read-only: Operational means at least one post or
// comment has been ingested for that platform, same signal dashboard's
// DataSourceStatus.Active already uses — there is no admin override. Once
// Argus exposes real crawler health, swap this for that instead.
func (s *settingsService) GetIntegrations(ctx context.Context) ([]IntegrationItem, error) {
	volume, err := s.postRepo.CombinedPlatformVolume(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	countByPlatform := make(map[model.Platform]int64, len(volume))
	for _, v := range volume {
		countByPlatform[v.Platform] = v.Total
	}

	items := make([]IntegrationItem, 0, len(integrationPlatforms))
	for _, platform := range integrationPlatforms {
		count := countByPlatform[platform]
		status := model.IntegrationNotOperational
		if count > 0 {
			status = model.IntegrationOperational
		}
		items = append(items, IntegrationItem{
			Platform: platform,
			Status:   status,
			Count:    count,
		})
	}
	return items, nil
}
