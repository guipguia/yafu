package api

import (
	"encoding/json"
	"net/http"

	"github.com/guipguia/yafu/internal/auth"
)

type whoamiResponse struct {
	Subject     string   `json:"subject"`
	Email       string   `json:"email,omitempty"`
	Name        string   `json:"name,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	IsAnonymous bool     `json:"isAnonymous"`
}

func handleWhoami(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		// Should never happen — middleware always sets identity.
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no identity in context"})
		return
	}
	_ = json.NewEncoder(w).Encode(whoamiResponse{
		Subject:     id.Subject,
		Email:       id.Email,
		Name:        id.Name,
		Groups:      id.Groups,
		IsAnonymous: id.IsAnonymous(),
	})
}
