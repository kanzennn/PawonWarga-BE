package model

// IntegrationStatus is a platform's connectivity display value on
// /settings/integrations. It is read-only and computed (see
// service.SettingsService.GetIntegrations) from whether any data has
// actually been ingested for that platform — the same signal dashboard's
// DataSourceStatus.Active already uses. Not persisted: there is no admin
// toggle. Once Argus exposes real crawler health, this should be swapped to
// read that instead of inferring from data presence.
type IntegrationStatus string

const (
	IntegrationOperational    IntegrationStatus = "Operational"
	IntegrationNotOperational IntegrationStatus = "Not Operational"
)

// UserPreference is one user's override for a named monitoring toggle. The
// catalog of valid keys plus their title/desc/default lives in
// service.PreferenceCatalog, not the database, so copy can change without a
// migration — a user with no row for a key gets the catalog default.
type UserPreference struct {
	BaseModel
	UserID  uint   `gorm:"uniqueIndex:idx_user_preference_key;not null"`
	Key     string `gorm:"type:varchar(64);uniqueIndex:idx_user_preference_key;not null"`
	Checked bool   `gorm:"not null"`
}
