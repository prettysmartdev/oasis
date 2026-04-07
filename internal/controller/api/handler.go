// Package api implements the oasis management HTTP API handlers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	agentpkg "github.com/prettysmartdev/oasis/internal/controller/agent"
	"github.com/prettysmartdev/oasis/internal/controller/db"
	"github.com/prettysmartdev/oasis/internal/controller/nginx"
	tsnetpkg "github.com/prettysmartdev/oasis/internal/controller/tsnet"
)

// TsnetNode is the subset of tsnet.Node used by the API handler.
type TsnetNode interface {
	IsStarted() bool
	TailscaleIP() (string, error)
	TailscaleDNSName(ctx context.Context) (string, error)
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
	onSetup  func(net.Listener) // called with the tsnet listener after first-run setup
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

// SetOnSetup registers a callback invoked with the tsnet listener after a successful
// first-run setup. main.go uses this to start the webapp API server without the
// management handler needing to know about HTTP server lifecycle details.
func (h *Handler) SetOnSetup(fn func(net.Listener)) {
	h.onSetup = fn
}

// RegisterRoutes registers all API routes on the provided mux.
// Write-mutating routes are omitted when readOnly is true.
// For the tsnet (readOnly) handler a catch-all reverse proxy to NGINX is added so
// that the static webapp and app upstreams are reachable via the Tailscale network.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/status", h.handleStatus)
	mux.HandleFunc("GET /api/v1/apps", h.handleListApps)
	mux.HandleFunc("GET /api/v1/apps/{slug}", h.handleGetApp)
	mux.HandleFunc("GET /api/v1/settings", h.handleGetSettings)

	// Agent read-only endpoints (available on both management and tsnet handlers).
	// Register /api/v1/agents/runs/{runId} before /api/v1/agents/{slug} so the
	// more-specific literal segment "runs" wins in Go 1.22+ ServeMux matching.
	mux.HandleFunc("GET /api/v1/agents/runs/{runId}", h.handleGetAgentRun)
	mux.HandleFunc("GET /api/v1/agents", h.handleListAgents)
	mux.HandleFunc("GET /api/v1/agents/{slug}", h.handleGetAgent)
	mux.HandleFunc("GET /api/v1/agents/{slug}/runs/latest", h.handleGetLatestAgentRun)
	mux.HandleFunc("POST /api/v1/agents/{slug}/webhook", h.handleAgentWebhook)

	if !h.readOnly {
		mux.HandleFunc("POST /api/v1/apps", h.handleCreateApp)
		mux.HandleFunc("PATCH /api/v1/apps/{slug}", h.handleUpdateApp)
		mux.HandleFunc("DELETE /api/v1/apps/{slug}", h.handleDeleteApp)
		mux.HandleFunc("POST /api/v1/apps/{slug}/enable", h.handleEnableApp)
		mux.HandleFunc("POST /api/v1/apps/{slug}/disable", h.handleDisableApp)
		mux.HandleFunc("PATCH /api/v1/settings", h.handleUpdateSettings)
		mux.HandleFunc("POST /api/v1/setup", h.handleSetup)

		// Agent write endpoints (management only).
		mux.HandleFunc("POST /api/v1/agents", h.handleCreateAgent)
		mux.HandleFunc("PATCH /api/v1/agents/{slug}", h.handleUpdateAgent)
		mux.HandleFunc("DELETE /api/v1/agents/{slug}", h.handleDeleteAgent)
		mux.HandleFunc("POST /api/v1/agents/{slug}/enable", h.handleEnableAgent)
		mux.HandleFunc("POST /api/v1/agents/{slug}/disable", h.handleDisableAgent)
		mux.HandleFunc("POST /api/v1/agents/{slug}/run", h.handleTriggerAgentRun)
	} else {
		// Tsnet handler: reverse-proxy anything that isn't an /api/v1/ route to
		// NGINX (static webapp assets, /apps/<slug>/ upstreams). The more-specific
		// /api/v1/ patterns above always take precedence over this catch-all.
		nginxURL, _ := url.Parse(nginx.LocalAddr)
		mux.Handle("/", httputil.NewSingleHostReverseProxy(nginxURL))
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
// Silently skips if store, nginx, or the tsnet node is unavailable or not yet started.
func (h *Handler) triggerNginxReload(ctx context.Context) {
	if h.store == nil || h.nginx == nil || h.node == nil {
		return
	}
	if _, err := h.node.TailscaleIP(); err != nil {
		// Node not started yet — skip reload.
		return
	}
	apps, err := h.store.ListApps(ctx)
	if err != nil {
		slog.Warn("nginx reload: list apps failed", "err", err)
		return
	}
	if err := h.nginx.Apply(ctx, apps); err != nil {
		slog.Warn("nginx reload failed", "err", err)
	}
}

// --- Status ------------------------------------------------------------------

type statusResponse struct {
	TailscaleConnected bool   `json:"tailscaleConnected"`
	TailscaleIP        string `json:"tailscaleIP"`
	TailscaleHostname  string `json:"tailscaleHostname"`
	TailscaleDNSName   string `json:"tailscaleDNSName"`
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
		if dnsName, err := h.node.TailscaleDNSName(r.Context()); err == nil {
			resp.TailscaleDNSName = dnsName
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

	ln, err := h.node.Start(r.Context())
	if err != nil {
		var conflictErr *tsnetpkg.HostnameConflictError
		if errors.As(err, &conflictErr) {
			writeError(w, http.StatusConflict, conflictErr.Error(), "HOSTNAME_CONFLICT")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to start tsnet node", "TSNET_START_FAILED")
		return
	}
	if h.onSetup != nil {
		h.onSetup(ln)
	}

	// Return current status.
	resp := statusResponse{Version: h.version, TailscaleConnected: true}
	if ip, err := h.node.TailscaleIP(); err == nil {
		resp.TailscaleIP = ip
	}
	if dnsName, err := h.node.TailscaleDNSName(r.Context()); err == nil {
		resp.TailscaleDNSName = dnsName
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

// --- Agents ------------------------------------------------------------------

// agentJSON is the wire representation of an Agent.
type agentJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Prompt      string `json:"prompt"`
	Trigger     string `json:"trigger"`
	Schedule    string `json:"schedule"`
	OutputFmt   string `json:"outputFmt"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// agentRunJSON is the wire representation of an AgentRun.
type agentRunJSON struct {
	ID         string  `json:"id"`
	AgentID    string  `json:"agentId"`
	TriggerSrc string  `json:"triggerSrc"`
	Status     string  `json:"status"`
	Output     string  `json:"output"`
	StartedAt  string  `json:"startedAt"`
	FinishedAt *string `json:"finishedAt"`
}

func toAgentJSON(a db.Agent) agentJSON {
	return agentJSON{
		ID:          a.ID,
		Name:        a.Name,
		Slug:        a.Slug,
		Description: a.Description,
		Icon:        a.Icon,
		Prompt:      a.Prompt,
		Trigger:     a.Trigger,
		Schedule:    a.Schedule,
		OutputFmt:   a.OutputFmt,
		Enabled:     a.Enabled,
		CreatedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAgentRunJSON(r db.AgentRun) agentRunJSON {
	j := agentRunJSON{
		ID:         r.ID,
		AgentID:    r.AgentID,
		TriggerSrc: r.TriggerSrc,
		Status:     r.Status,
		Output:     r.Output,
		StartedAt:  r.StartedAt.UTC().Format(time.RFC3339),
	}
	if r.FinishedAt != nil {
		s := r.FinishedAt.UTC().Format(time.RFC3339)
		j.FinishedAt = &s
	}
	return j
}

// isValidCronExpr reports whether expr is a valid 5-field cron expression.
func isValidCronExpr(expr string) bool {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(expr)
	return err == nil
}

func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	agents, err := h.store.ListAgents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agents", "INTERNAL_ERROR")
		return
	}
	items := make([]agentJSON, 0, len(agents))
	for _, a := range agents {
		items = append(items, toAgentJSON(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	a, err := h.store.GetAgent(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toAgentJSON(*a))
}

type createAgentRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Prompt      string `json:"prompt"`
	Trigger     string `json:"trigger"`
	Schedule    string `json:"schedule"`
	OutputFmt   string `json:"outputFmt"`
	Enabled     *bool  `json:"enabled"` // defaults to true when omitted
}

func (h *Handler) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	var req createAgentRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required", "INVALID_NAME")
		return
	}
	if !slugRe.MatchString(req.Slug) {
		writeError(w, http.StatusBadRequest, "slug must match [a-z0-9-]+", "INVALID_SLUG")
		return
	}
	validTriggers := map[string]bool{"tap": true, "schedule": true, "webhook": true}
	if !validTriggers[req.Trigger] {
		writeError(w, http.StatusBadRequest, "trigger must be one of: tap, schedule, webhook", "INVALID_TRIGGER")
		return
	}
	if req.Trigger == "schedule" {
		if req.Schedule == "" {
			writeError(w, http.StatusBadRequest, "schedule is required when trigger is 'schedule'", "INVALID_SCHEDULE")
			return
		}
		if !isValidCronExpr(req.Schedule) {
			writeError(w, http.StatusBadRequest, "schedule must be a valid 5-field cron expression", "INVALID_SCHEDULE")
			return
		}
	}
	if req.OutputFmt == "" {
		req.OutputFmt = "markdown"
	}
	validFmts := map[string]bool{"markdown": true, "html": true, "plaintext": true}
	if !validFmts[req.OutputFmt] {
		writeError(w, http.StatusBadRequest, "outputFmt must be one of: markdown, html, plaintext", "INVALID_OUTPUT_FMT")
		return
	}

	enabled := true // default: agents are enabled on creation
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now().UTC()
	agent := db.Agent{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		Icon:        req.Icon,
		Prompt:      req.Prompt,
		Trigger:     req.Trigger,
		Schedule:    req.Schedule,
		OutputFmt:   req.OutputFmt,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.CreateAgent(r.Context(), agent); err != nil {
		if isUniqueConstraintError(err) {
			writeError(w, http.StatusConflict, "slug already exists", "SLUG_CONFLICT")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create agent", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, toAgentJSON(agent))
}

type updateAgentRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Prompt      *string `json:"prompt"`
	Trigger     *string `json:"trigger"`
	Schedule    *string `json:"schedule"`
	OutputFmt   *string `json:"outputFmt"`
	Enabled     *bool   `json:"enabled"`
}

func (h *Handler) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")

	// Verify the agent exists.
	if _, err := h.store.GetAgent(r.Context(), slug); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent", "INTERNAL_ERROR")
		return
	}

	var req updateAgentRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	if req.Trigger != nil {
		validTriggers := map[string]bool{"tap": true, "schedule": true, "webhook": true}
		if !validTriggers[*req.Trigger] {
			writeError(w, http.StatusBadRequest, "trigger must be one of: tap, schedule, webhook", "INVALID_TRIGGER")
			return
		}
	}
	if req.OutputFmt != nil {
		validFmts := map[string]bool{"markdown": true, "html": true, "plaintext": true}
		if !validFmts[*req.OutputFmt] {
			writeError(w, http.StatusBadRequest, "outputFmt must be one of: markdown, html, plaintext", "INVALID_OUTPUT_FMT")
			return
		}
	}
	// When changing trigger to "schedule", a valid cron expression is required.
	if req.Trigger != nil && *req.Trigger == "schedule" {
		schedule := ""
		if req.Schedule != nil {
			schedule = *req.Schedule
		}
		if schedule == "" {
			writeError(w, http.StatusBadRequest, "schedule is required when trigger is 'schedule'", "INVALID_SCHEDULE")
			return
		}
		if !isValidCronExpr(schedule) {
			writeError(w, http.StatusBadRequest, "schedule must be a valid 5-field cron expression", "INVALID_SCHEDULE")
			return
		}
	}

	fields := make(map[string]any)
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.Icon != nil {
		fields["icon"] = *req.Icon
	}
	if req.Prompt != nil {
		fields["prompt"] = *req.Prompt
	}
	if req.Trigger != nil {
		fields["trigger"] = *req.Trigger
	}
	if req.Schedule != nil {
		fields["schedule"] = *req.Schedule
	}
	if req.OutputFmt != nil {
		fields["outputFmt"] = *req.OutputFmt
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}

	if err := h.store.UpdateAgent(r.Context(), slug, fields); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent", "INTERNAL_ERROR")
		return
	}

	updated, err := h.store.GetAgent(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated agent", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toAgentJSON(*updated))
}

func (h *Handler) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	if err := h.store.DeleteAgent(r.Context(), slug); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete agent", "INTERNAL_ERROR")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleEnableAgent(w http.ResponseWriter, r *http.Request) {
	h.setAgentEnabled(w, r, true)
}

func (h *Handler) handleDisableAgent(w http.ResponseWriter, r *http.Request) {
	h.setAgentEnabled(w, r, false)
}

func (h *Handler) setAgentEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	if err := h.store.UpdateAgent(r.Context(), slug, map[string]any{"enabled": enabled}); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update agent", "INTERNAL_ERROR")
		return
	}
	updated, err := h.store.GetAgent(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch agent", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toAgentJSON(*updated))
}

// handleTriggerAgentRun handles POST /api/v1/agents/{slug}/run (management only).
func (h *Handler) handleTriggerAgentRun(w http.ResponseWriter, r *http.Request) {
	h.triggerRun(w, r, "tap")
}

// handleAgentWebhook handles POST /api/v1/agents/{slug}/webhook.
func (h *Handler) handleAgentWebhook(w http.ResponseWriter, r *http.Request) {
	h.triggerRun(w, r, "webhook")
}

// triggerRun is the shared implementation for tap and webhook triggers.
func (h *Handler) triggerRun(w http.ResponseWriter, r *http.Request, triggerSrc string) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	a, err := h.store.GetAgent(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent", "INTERNAL_ERROR")
		return
	}

	// Check for an already-running run.
	existing, err := h.store.GetRunningAgentRun(r.Context(), a.ID)
	if err == nil && existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "a run is already in progress for this agent",
			"code":  "RUN_IN_PROGRESS",
			"runId": existing.ID,
		})
		return
	}

	runID := uuid.New().String()
	run := db.AgentRun{
		ID:         runID,
		AgentID:    a.ID,
		TriggerSrc: triggerSrc,
		Status:     "running",
		StartedAt:  time.Now().UTC(),
	}
	if err := h.store.CreateAgentRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create run", "INTERNAL_ERROR")
		return
	}

	agent := *a
	store := h.store
	go func() {
		output, runErr := agentpkg.Run(context.Background(), agent)
		status := "done"
		if runErr != nil {
			status = "error"
			output = runErr.Error()
		}
		finishedAt := time.Now().UTC()
		if updateErr := store.UpdateAgentRun(context.Background(), runID, status, output, finishedAt); updateErr != nil {
			slog.Error("failed to update agent run", "runId", runID, "err", updateErr)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"runId": runID})
}

func (h *Handler) handleGetAgentRun(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	runID := r.PathValue("runId")
	run, err := h.store.GetAgentRun(r.Context(), runID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get run", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toAgentRunJSON(*run))
}

func (h *Handler) handleGetLatestAgentRun(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not initialised", "STORE_UNAVAILABLE")
		return
	}
	slug := r.PathValue("slug")
	a, err := h.store.GetAgent(r.Context(), slug)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent not found", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get agent", "INTERNAL_ERROR")
		return
	}
	run, err := h.store.GetLatestAgentRun(r.Context(), a.ID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no runs found for this agent", "NOT_FOUND")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get run", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, toAgentRunJSON(*run))
}
