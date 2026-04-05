// Package api implements the oasis management HTTP API handlers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
)

// TsnetNode is the subset of tsnet.Node used by the API handler.
type TsnetNode interface {
	IsStarted() bool
	TailscaleIP() (string, error)
	Start(ctx context.Context) (net.Listener, error)
}

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// Handler holds the dependencies for the management API.
type Handler struct {
	store    *db.Store
	nginx    *nginx.Configurator
	node     TsnetNode
	readOnly bool
	version  string
}

// New creates a new Handler. Pass nil for dependencies not yet available (e.g. in tests).
func New(store *db.Store, configurator *nginx.Configurator, node TsnetNode, readOnly bool) *Handler {
	return &Handler{
		store:    store,
		nginx:    configurator,
		node:     node,
		readOnly: readOnly,
	}
}

// SetVersion sets the version string returned by the status endpoint.
func (h *Handler) SetVersion(v string) {
	h.version = v
}

// RegisterRoutes registers all API routes on the provided mux.
// Write-mutating routes are omitted when readOnly is true.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/status", h.handleStatus)
	mux.HandleFunc("GET /api/v1/apps", h.handleListApps)
	mux.HandleFunc("GET /api/v1/apps/{slug}", h.handleGetApp)
	mux.HandleFunc("GET /api/v1/settings", h.handleGetSettings)

	if !h.readOnly {
		mux.HandleFunc("POST /api/v1/apps", h.handleCreateApp)
		mux.HandleFunc("PATCH /api/v1/apps/{slug}", h.handleUpdateApp)
		mux.HandleFunc("DELETE /api/v1/apps/{slug}", h.handleDeleteApp)
		mux.HandleFunc("POST /api/v1/apps/{slug}/enable", h.handleEnableApp)
		mux.HandleFunc("POST /api/v1/apps/{slug}/disable", h.handleDisableApp)
		mux.HandleFunc("PATCH /api/v1/settings", h.handleUpdateSettings)
		mux.HandleFunc("POST /api/v1/setup", h.handleSetup)
	}
}

// --- Helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, humanMsg, code string) {
	writeJSON(w, status, map[string]string{
		"error": humanMsg,
		"code":  code,
	})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// appJSON is the wire representation of an App.
type appJSON struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	UpstreamURL string   `json:"upstreamURL"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Tags        []string `json:"tags"`
	Enabled     bool     `json:"enabled"`
	Health      string   `json:"health"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

func toAppJSON(a db.App) appJSON {
	tags := a.Tags
	if tags == nil {
		tags = []string{}
	}
	return appJSON{
		ID:          a.ID,
		Name:        a.Name,
		Slug:        a.Slug,
		UpstreamURL: a.UpstreamURL,
		DisplayName: a.DisplayName,
		Description: a.Description,
		Icon:        a.Icon,
		Tags:        tags,
		Enabled:     a.Enabled,
		Health:      a.Health,
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// triggerNginxReload re-generates and applies the NGINX config.
// Silently skips if store, nginx, or the tsnet node is unavailable.
func (h *Handler) triggerNginxReload(ctx context.Context) {
	if h.store == nil || h.nginx == nil || h.node == nil {
		return
	}
	apps, err := h.store.ListApps(ctx)
	if err != nil {
		slog.Warn("nginx reload: list apps failed", "err", err)
		return
	}
	ip, err := h.node.TailscaleIP()
	if err != nil {
		// Node not started yet — skip reload.
		return
	}
	if err := h.nginx.Apply(ctx, apps, ip); err != nil {
		slog.Warn("nginx reload failed", "err", err)
	}
}

// --- Status ------------------------------------------------------------------

type statusResponse struct {
	TailscaleConnected bool   `json:"tailscaleConnected"`
	TailscaleIP        string `json:"tailscaleIP"`
	TailscaleHostname  string `json:"tailscaleHostname"`
	NginxStatus        string `json:"nginxStatus"`
	RegisteredAppCount int    `json:"registeredAppCount"`
	Version            string `json:"version"`
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := statusResponse{Version: h.version}

	if h.node != nil && h.node.IsStarted() {
		resp.TailscaleConnected = true
		if ip, err := h.node.TailscaleIP(); err == nil {
			resp.TailscaleIP = ip
		}
	}

	if h.store != nil {
		settings, err := h.store.GetSettings(r.Context())
		if err == nil {
			resp.TailscaleHostname = settings.TailscaleHostname
		}
		apps, err := h.store.ListApps(r.Context())
		if err == nil {
			resp.RegisteredAppCount = len(apps)
		}
	}

	if _, err := nginx.FindNginxPID(); err == nil {
		resp.NginxStatus = "running"
	} else {
		resp.NginxStatus = "stopped"
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Apps --------------------------------------------------------------------

func (h *Handler) handleListApps(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	apps, err := h.store.ListApps(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list apps", "INTERNAL_ERROR")
		return
	}
	items := make([]appJSON, 0, len(apps))
	for _, a := range apps {
		items = append(items, toAppJSON(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

type createAppRequest struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	UpstreamURL string   `json:"upstreamURL"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
	Tags        []string `json:"tags"`
	Enabled     bool     `json:"enabled"`
}

func (h *Handler) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	var req createAppRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if !slugRe.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "slug must match [a-z0-9-]+", "INVALID_SLUG")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "INVALID_NAME")
		return
	}
	if !isValidUpstreamURL(req.UpstreamURL) {
		writeError(w, http.StatusBadRequest, "upstreamURL must be a valid http or https URL with a host", "INVALID_UPSTREAM_URL")
		return
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	now := time.Now().UTC()
	app := db.App{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Slug:        req.Slug,
		UpstreamURL: req.UpstreamURL,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Icon:        req.Icon,
		Tags:        tags,
		Enabled:     req.Enabled,
		Health:      "unknown",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateApp(r.Context(), app); err != nil {
		if isUniqueConstraintError(err) {
			writeError(w, http.StatusConflict, "slug already exists", "SLUG_CONFLICT")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create app", "INTERNAL_ERROR")
		return
	}
	h.triggerNginxReload(r.Context())
	writeJSON(w, http.StatusCreated, toAppJSON(app))
}

func (h *Handler) handleGetApp(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	app, err := h.store.GetApp(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get app", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toAppJSON(app))
}

type updateAppRequest struct {
	Name        *string   `json:"name"`
	UpstreamURL *string   `json:"upstreamURL"`
	DisplayName *string   `json:"displayName"`
	Description *string   `json:"description"`
	Icon        *string   `json:"icon"`
	Tags        *[]string `json:"tags"`
	Enabled     *bool     `json:"enabled"`
}

func (h *Handler) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")

	// Capture pre-update state to detect routing-relevant changes.
	before, err := h.store.GetApp(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get app", "INTERNAL_ERROR")
		return
	}

	var req updateAppRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	patch := db.AppPatch{
		Name:        req.Name,
		UpstreamURL: req.UpstreamURL,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Icon:        req.Icon,
		Tags:        req.Tags,
		Enabled:     req.Enabled,
	}
	updated, err := h.store.UpdateApp(r.Context(), slug, patch)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update app", "INTERNAL_ERROR")
		return
	}

	// Reload NGINX if routing-relevant fields changed.
	if req.Enabled != nil || req.UpstreamURL != nil ||
		(req.Enabled == nil && updated.Enabled != before.Enabled) {
		h.triggerNginxReload(r.Context())
	}
	writeJSON(w, http.StatusOK, toAppJSON(updated))
}

func (h *Handler) handleDeleteApp(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	if err := h.store.DeleteApp(r.Context(), slug); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found", "NOT_FOUND")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete app", "INTERNAL_ERROR")
		return
	}
	h.triggerNginxReload(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleEnableApp(w http.ResponseWriter, r *http.Request) {
	h.setAppEnabled(w, r, true)
}

func (h *Handler) handleDisableApp(w http.ResponseWriter, r *http.Request) {
	h.setAppEnabled(w, r, false)
}

func (h *Handler) setAppEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	updated, err := h.store.UpdateApp(r.Context(), slug, db.AppPatch{Enabled: &enabled})
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update app", "INTERNAL_ERROR")
		return
	}
	h.triggerNginxReload(r.Context())
	writeJSON(w, http.StatusOK, toAppJSON(updated))
}

// --- Settings ----------------------------------------------------------------

type settingsJSON struct {
	TailscaleHostname string `json:"tailscaleHostname"`
	MgmtPort          int    `json:"mgmtPort"`
	Theme             string `json:"theme"`
}

func toSettingsJSON(s db.Settings) settingsJSON {
	return settingsJSON{
		TailscaleHostname: s.TailscaleHostname,
		MgmtPort:          s.MgmtPort,
		Theme:             s.Theme,
	}
}

func (h *Handler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toSettingsJSON(settings))
}

type updateSettingsRequest struct {
	TailscaleHostname *string `json:"tailscaleHostname"`
	MgmtPort          *int    `json:"mgmtPort"`
	Theme             *string `json:"theme"`
}

func (h *Handler) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	var req updateSettingsRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	updated, err := h.store.UpdateSettings(r.Context(), db.SettingsPatch{
		TailscaleHostname: req.TailscaleHostname,
		MgmtPort:          req.MgmtPort,
		Theme:             req.Theme,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toSettingsJSON(updated))
}

// --- Setup -------------------------------------------------------------------

type setupRequest struct {
	TailscaleAuthKey string `json:"tailscaleAuthKey"`
	Hostname         string `json:"hostname"`
}

func (h *Handler) handleSetup(w http.ResponseWriter, r *http.Request) {
	if h.node == nil {
		writeError(w, http.StatusServiceUnavailable, "tsnet node not initialised", "NODE_UNAVAILABLE")
		return
	}
	if h.node.IsStarted() {
		writeError(w, http.StatusConflict, "already configured", "ALREADY_CONFIGURED")
		return
	}

	var req setupRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	// Set TS_AUTHKEY without logging it.
	if req.TailscaleAuthKey != "" {
		os.Setenv("TS_AUTHKEY", req.TailscaleAuthKey) //nolint:errcheck
	}

	// Update hostname in settings if provided.
	if req.Hostname != "" && h.store != nil {
		h.store.UpdateSettings(r.Context(), db.SettingsPatch{ //nolint:errcheck
			TailscaleHostname: &req.Hostname,
		})
	}

	if _, err := h.node.Start(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start tsnet node", "TSNET_START_FAILED")
		return
	}

	// Return current status.
	resp := statusResponse{Version: h.version, TailscaleConnected: true}
	if ip, err := h.node.TailscaleIP(); err == nil {
		resp.TailscaleIP = ip
	}
	if h.store != nil {
		settings, err := h.store.GetSettings(r.Context())
		if err == nil {
			resp.TailscaleHostname = settings.TailscaleHostname
		}
		apps, err := h.store.ListApps(r.Context())
		if err == nil {
			resp.RegisteredAppCount = len(apps)
		}
	}
	if _, err := nginx.FindNginxPID(); err == nil {
		resp.NginxStatus = "running"
	} else {
		resp.NginxStatus = "stopped"
	}
	writeJSON(w, http.StatusOK, resp)
}

// isValidUpstreamURL reports whether s is a valid http or https URL with a non-empty host.
func isValidUpstreamURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// isUniqueConstraintError returns true if err is a SQLite UNIQUE constraint violation.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "unique constraint")
}
