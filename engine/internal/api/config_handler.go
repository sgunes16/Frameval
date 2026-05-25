package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Service) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.store.ListModelConfigs(r.Context())
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, models)
}

func (s *Service) ListAgents(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, s.store.ListAgents())
}

func (s *Service) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListAPIKeys(r.Context())
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, keys)
}

func (s *Service) UpsertAPIKey(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if err := s.store.UpsertAPIKey(r.Context(), payload.Provider, payload.APIKey); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusCreated, map[string]bool{"ok": true})
}

func (s *Service) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteAPIKey(r.Context(), chi.URLParam(r, "provider")); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// validJudgeProviders is the closed set the user may pick. Keep in sync
// with grader/llm_client.py's _PRESETS.
var validJudgeProviders = map[string]struct{}{
	"openrouter": {},
	"zai":        {},
	"ollama":     {},
	"openai":     {},
	"anthropic":  {},
}

type llmSettingsPayload struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Enabled       bool   `json:"enabled"`
	APIKeyPresent bool   `json:"api_key_present"`
}

func (s *Service) GetLLMSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetSettingsByPrefix(r.Context(), "judge.")
	if err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	provider := settings["judge.provider"]
	if provider == "" {
		provider = "openrouter"
	}
	model := settings["judge.model"]
	if model == "" {
		model = "deepseek/deepseek-chat-v3-0324:free"
	}
	enabled := settings["judge.enabled"] == "true"

	// api_key_present checks api_keys for the currently-selected provider.
	_, keyErr := s.store.GetDecryptedAPIKey(r.Context(), provider)
	keyPresent := keyErr == nil

	JSON(w, http.StatusOK, llmSettingsPayload{
		Provider:      provider,
		Model:         model,
		Enabled:       enabled,
		APIKeyPresent: keyPresent,
	})
}

func (s *Service) PutLLMSettings(w http.ResponseWriter, r *http.Request) {
	var payload llmSettingsPayload
	if err := decodeJSON(r, &payload); err != nil {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "invalid request", err)
		return
	}
	if _, ok := validJudgeProviders[payload.Provider]; !ok {
		renderError(w, r.Context(), http.StatusBadRequest, ErrCodeBadRequest, "unknown provider", nil)
		return
	}
	if err := s.store.SetSetting(r.Context(), "judge.provider", payload.Provider); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	if err := s.store.SetSetting(r.Context(), "judge.model", payload.Model); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	enabledStr := "false"
	if payload.Enabled {
		enabledStr = "true"
	}
	if err := s.store.SetSetting(r.Context(), "judge.enabled", enabledStr); err != nil {
		renderError(w, r.Context(), http.StatusInternalServerError, ErrCodeInternal, "internal error", err)
		return
	}
	// Reuse the GET path for the response body so it includes api_key_present.
	s.GetLLMSettings(w, r)
}
