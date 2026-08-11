package handler

import (
	"errors"

	"PawonWarga-BE/internal/service"
	"PawonWarga-BE/pkg/i18n"
	"PawonWarga-BE/pkg/response"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	settingsSvc service.SettingsService
}

func NewSettingsHandler(settingsSvc service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settingsSvc: settingsSvc}
}

type PreferenceResponse struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Desc    string `json:"desc"`
	Checked bool   `json:"checked"`
}

type IntegrationResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Count  string `json:"count"`
}

func toPreferenceResponse(items []service.PreferenceItem) []PreferenceResponse {
	out := make([]PreferenceResponse, len(items))
	for i, item := range items {
		out[i] = PreferenceResponse{Key: item.Key, Title: item.Title, Desc: item.Desc, Checked: item.Checked}
	}
	return out
}

func toIntegrationResponse(items []service.IntegrationItem) []IntegrationResponse {
	out := make([]IntegrationResponse, len(items))
	for i, item := range items {
		out[i] = IntegrationResponse{
			Name:   platformSourceLabel[item.Platform],
			Status: string(item.Status),
			Count:  formatCompact(item.Count) + " mentions",
		}
	}
	return out
}

// GetPreferences godoc
// @Summary      Get monitoring preferences
// @Description  Per-user notification toggles. title/desc come from a fixed backend catalog (service.PreferenceCatalog) so copy can change without a frontend redeploy — a user who has never touched a toggle gets the catalog default. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.ErrorResponse
// @Router       /settings/preferences [get]
func (h *SettingsHandler) GetPreferences(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := c.MustGet("user_id").(uint)

	items, err := h.settingsSvc.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "settings.preferences.get_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "settings.preferences.get_ok"), toPreferenceResponse(items))
}

type UpdatePreferenceRequest struct {
	Key     string `json:"key" binding:"required"`
	Checked bool   `json:"checked"`
}

// UpdatePreference godoc
// @Summary      Update a monitoring preference
// @Description  Flips one preference for the current user and returns the full updated list, same shape as GET. key must be one of the keys GET /settings/preferences returns.
// @Tags         settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      UpdatePreferenceRequest  true  "Preference key and new checked state"
// @Success      200  {object}  response.Response
// @Failure      400  {object}  response.ErrorResponse
// @Failure      500  {object}  response.ErrorResponse
// @Router       /settings/preferences [put]
func (h *SettingsHandler) UpdatePreference(c *gin.Context) {
	lang := i18n.FromContext(c)
	userID := c.MustGet("user_id").(uint)

	var req UpdatePreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationFailed(c, err)
		return
	}

	items, err := h.settingsSvc.UpdatePreference(c.Request.Context(), userID, req.Key, req.Checked)
	if err != nil {
		if errors.Is(err, service.ErrUnknownPreference) {
			response.BadRequest(c, i18n.T(lang, "settings.preferences.unknown_key"), err)
			return
		}
		response.InternalServerError(c, i18n.T(lang, "settings.preferences.update_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "settings.preferences.update_ok"), toPreferenceResponse(items))
}

// GetIntegrations godoc
// @Summary      Get data source integrations
// @Description  Every monitored platform's read-only connectivity status plus its live all-time mention count (posts+comments, never persisted so it can't go stale). Status is Operational whenever at least one item has been ingested for that platform — there is no manual override; once Argus exposes real crawler health this should read that instead. Response messages are localized via ?lang= or Accept-Language (id/en).
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.ErrorResponse
// @Router       /settings/integrations [get]
func (h *SettingsHandler) GetIntegrations(c *gin.Context) {
	lang := i18n.FromContext(c)

	items, err := h.settingsSvc.GetIntegrations(c.Request.Context())
	if err != nil {
		response.InternalServerError(c, i18n.T(lang, "settings.integrations.get_failed"), err)
		return
	}

	response.OK(c, i18n.T(lang, "settings.integrations.get_ok"), toIntegrationResponse(items))
}
