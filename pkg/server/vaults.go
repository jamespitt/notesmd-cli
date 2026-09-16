package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Yakitrak/notesmd-cli/pkg/obsidian"
)

// Vault is one vault this server can serve. Clients pick one per request with
// ?vault=<id> (or an X-Vault header); the first registered vault is the
// default when they pick none, which is what keeps single-vault clients -
// every client written before vault switching existed - working unchanged.
type Vault struct {
	ID      string
	Label   string
	Manager obsidian.VaultManager

	// StateKey namespaces the server-side state that isn't stored in the
	// vault itself (hidden calendar events). It's empty for the default
	// vault so that state keeps the filename it has always had.
	StateKey string
}

// DefaultVaultID is the id given to the single vault of a server started
// without any named vaults (`notesmd-cli serve` with no "vaults" config).
const DefaultVaultID = "default"

type vaultCtxKeyType struct{}

var vaultCtxKey = vaultCtxKeyType{}

// New returns a single-vault server - the vault is reachable both as the
// default (no ?vault= param) and by the id "default".
func New(vault obsidian.VaultManager, note obsidian.NoteManager) *Server {
	return NewMulti([]Vault{{ID: DefaultVaultID, Label: "Default", Manager: vault}}, note)
}

// NewMulti returns a server over one or more named vaults. The first vault in
// the slice is the default. StateKey is filled in here rather than by callers:
// the default vault keeps the unnamespaced state files, the rest are keyed by
// id.
func NewMulti(vaults []Vault, note obsidian.NoteManager) *Server {
	registered := make([]Vault, 0, len(vaults))
	for i, v := range vaults {
		if i == 0 {
			v.StateKey = ""
		} else {
			v.StateKey = v.ID
		}
		if v.Label == "" {
			v.Label = v.ID
		}
		registered = append(registered, v)
	}
	return &Server{vaults: registered, note: note}
}

// VaultsFromConfig turns preferences.json "vaults" entries into servable
// vaults, in config order.
func VaultsFromConfig(configs []obsidian.VaultConfig) []Vault {
	out := make([]Vault, 0, len(configs))
	for _, c := range configs {
		out = append(out, Vault{
			ID:      c.ID,
			Label:   c.DisplayLabel(),
			Manager: obsidian.NewConfiguredVault(c),
		})
	}
	return out
}

// lookupVault resolves a vault id; an empty id means the default vault.
func (s *Server) lookupVault(id string) (Vault, bool) {
	if len(s.vaults) == 0 {
		return Vault{}, false
	}
	if id == "" {
		return s.vaults[0], true
	}
	for _, v := range s.vaults {
		if v.ID == id {
			return v, true
		}
	}
	return Vault{}, false
}

// withVault resolves the request's vault once, up front, and stashes it on the
// context for the handlers to read via vaultOf/vaultTarget. An unknown id is
// rejected here rather than falling back to the default - quietly writing a
// work task into the personal vault is worse than a failed request.
func (s *Server) withVault(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("vault")
		if id == "" {
			id = r.Header.Get("X-Vault")
		}

		vault, ok := s.lookupVault(id)
		if !ok {
			jsonError(w, http.StatusNotFound, fmt.Sprintf("unknown vault %q", id))
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), vaultCtxKey, vault)))
	})
}

// vaultTarget returns the vault resolved for this request by withVault.
func (s *Server) vaultTarget(r *http.Request) Vault {
	if v, ok := r.Context().Value(vaultCtxKey).(Vault); ok {
		return v
	}
	// Only reachable if a handler is called outside withVault.
	if len(s.vaults) > 0 {
		return s.vaults[0]
	}
	return Vault{}
}

// vaultOf returns the VaultManager for this request's vault.
func (s *Server) vaultOf(r *http.Request) obsidian.VaultManager {
	return s.vaultTarget(r).Manager
}

// GET /api/vaults
func (s *Server) listVaults(w http.ResponseWriter, r *http.Request) {
	type vaultInfo struct {
		ID      string `json:"id"`
		Label   string `json:"label"`
		Default bool   `json:"default,omitempty"`
	}

	active := s.vaultTarget(r)
	out := make([]vaultInfo, 0, len(s.vaults))
	for i, v := range s.vaults {
		out = append(out, vaultInfo{ID: v.ID, Label: v.Label, Default: i == 0})
	}

	jsonOK(w, map[string]any{"vaults": out, "active": active.ID})
}
