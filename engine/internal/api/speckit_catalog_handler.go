package api

import (
	"net/http"

	"github.com/mustafaselman/frameval/engine/internal/builtin/speckit"
)

// SpecKitExtensionPublic is the wire shape exposed to the frontend. It
// clips Stage.PromptTemplate (server-only — keeps prompt engineering
// out of public API surface) but carries everything the launcher needs
// to render the catalog picker.
type SpecKitExtensionPublic struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Stages      []StagePublic `json:"stages"`
	MultiAgent  bool          `json:"multi_agent"`
	SourceURL   string        `json:"source_url,omitempty"`
}

type StagePublic struct {
	Name         string `json:"name"`
	SlashCommand string `json:"slash_command"`
	Role         string `json:"role,omitempty"`
}

// ListSpecKitCatalog returns every curated extension. Catalog is static
// within a process so no caching concerns at this layer.
func (s *Service) ListSpecKitCatalog(w http.ResponseWriter, _ *http.Request) {
	entries := speckit.List()
	out := make([]SpecKitExtensionPublic, 0, len(entries))
	for _, e := range entries {
		stages := make([]StagePublic, 0, len(e.Stages))
		for _, st := range e.Stages {
			stages = append(stages, StagePublic{
				Name:         st.Name,
				SlashCommand: st.SlashCommand,
				Role:         st.Role,
			})
		}
		out = append(out, SpecKitExtensionPublic{
			ID:          e.ID,
			Name:        e.Name,
			Description: e.Description,
			Stages:      stages,
			MultiAgent:  e.MultiAgent,
			SourceURL:   e.SourceURL,
		})
	}
	JSON(w, http.StatusOK, out)
}
