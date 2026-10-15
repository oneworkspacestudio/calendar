package app

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/calendar/shared"
	"github.com/oneworkspacestudio/calendar/shared/secure"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var web embed.FS
var Version = "0.1.0-preview.1"

type Config struct{ Issuer, ClientID, ClientSecret, Origin, OwnerSubject, Domain string }
type Session struct {
	Auth, Subject, Name, Email, CSRF string
	Expires, TokenExpiry             time.Time
	RefreshToken                     string
	mu                               sync.Mutex
}

func (s *Session) auth() string { s.mu.Lock(); defer s.mu.Unlock(); return s.Auth }

type App struct {
	cfg                    Config
	kind, base, root, data string
	mu                     sync.RWMutex
	sessions               map[string]*Session
	pending                map[string]Login
	client                 *http.Client
	demo                   bool
	demoMu                 sync.Mutex
	demoData               map[string]DAVItem
	demoCalendars          []DAVItem
	listen                 string
	setupToken             string
}
type Options struct {
	Kind, Root, Data, Listen, Origin, Base, Manager string
	Demo, NoBrowser                                 bool
}

func Run(o Options) error {
	if o.Kind != "contacts" {
		o.Kind = "calendar"
	}
	if o.Demo && o.Root == "" {
		o.Root = filepath.Join(os.Getenv("LOCALAPPDATA"), "OneWorkspace", "CalendarDemoShared")
	}
	if o.Root == "" {
		if len(shared.DiscoverManaged()) > 1 {
			return errors.New("Several managed workspaces exist. Choose this app installation with --shared-data PATH.")
		}
		o.Root = shared.DefaultRoot()
	}
	if o.Listen == "" {
		o.Listen = "127.0.0.1:8998"
	}
	host, _, e := net.SplitHostPort(o.Listen)
	if e != nil || host != "127.0.0.1" {
		return errors.New("applications must listen on 127.0.0.1; use the authenticated suite proxy for public access")
	}
	if o.Origin == "" {
		o.Origin = "http://" + o.Listen
	}
	if !shared.ValidURL(o.Origin) || strings.TrimRight(o.Base, "/") != "" && (!strings.HasPrefix(o.Base, "/") || strings.Contains(o.Base, "..")) {
		return errors.New("invalid application origin/base path")
	}
	if !o.Demo {
		if e = shared.Prepare(o.Root); e != nil {
			return e
		}
	}
	if o.Data == "" {
		o.Data = filepath.Join(o.Root, "apps", o.Kind)
	}
	if e = os.MkdirAll(o.Data, 0700); e != nil {
		return e
	}
	if e = secure.ProtectDir(o.Data); e != nil {
		return e
	}
	a := &App{kind: o.Kind, root: o.Root, data: o.Data, base: strings.TrimRight(o.Base, "/"), sessions: map[string]*Session{}, pending: map[string]Login{}, client: &http.Client{Timeout: 40 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, demo: o.Demo, demoData: map[string]DAVItem{}, setupToken: secure.Token()}
	a.listen = o.Listen
	a.cfg.Origin = strings.TrimRight(o.Origin, "/")
	var sharedConfig shared.Config
	var loadErr error
	if o.Demo {
		loadErr = os.ErrNotExist
	} else {
		sharedConfig, _, loadErr = shared.Load(o.Root)
	}
	if loadErr == nil {
		a.cfg.Issuer = sharedConfig.Issuer
		a.cfg.ClientID = sharedConfig.ClientID
		a.cfg.ClientSecret = sharedConfig.ClientSecret
		a.cfg.Domain = sharedConfig.Domain
		if !o.Demo {
			_, e = shared.Install(o.Root, o.Kind, Version, nil)
			if e != nil {
				return e
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			e = shared.Ensure(ctx, o.Root, o.Manager)
			cancel()
			if e != nil {
				return e
			}
		}
	}
	if !o.Demo && loadErr != nil && !os.IsNotExist(loadErr) {
		return loadErr
	}
	_ = secure.SaveSecret(filepath.Join(o.Data, "setup.dpapi"), map[string]string{"url": a.cfg.Origin + a.base + "/setup#" + a.setupToken})
	if o.Demo {
		a.cfg.Domain = "example.test"
		a.seedDemo()
	}
	listener, e := net.Listen("tcp4", o.Listen)
	if e != nil {
		return e
	}
	server := &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		stop, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		server.Shutdown(stop)
	}()
	link := a.cfg.Origin + a.base + "/"
	if loadErr != nil && !o.Demo {
		link = "http://" + o.Listen + a.base + "/setup#" + a.setupToken
	}
	log.Print("My " + o.Kind + " listening at " + link)
	if !o.NoBrowser {
		secure.OpenBrowser(link)
	}
	e = server.Serve(listener)
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"status": "ok", "kind": a.kind, "version": Version})
	})
	mux.HandleFunc("GET /auth/login", a.login)
	mux.HandleFunc("GET /auth/callback", a.callback)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("GET /api/session", a.sessionInfo)
	mux.HandleFunc("GET /api/calendars", a.calendars)
	mux.HandleFunc("POST /api/calendars", a.createCalendar)
	mux.HandleFunc("GET /api/events", a.events)
	mux.HandleFunc("PUT /api/events", a.putEvent)
	mux.HandleFunc("DELETE /api/events", a.deleteEvent)
	mux.HandleFunc("GET /api/contacts", a.contacts)
	mux.HandleFunc("PUT /api/contacts", a.putContact)
	mux.HandleFunc("DELETE /api/contacts", a.deleteContact)
	mux.HandleFunc("GET /api/preferences", a.preferences)
	mux.HandleFunc("PUT /api/preferences", a.preferences)
	mux.HandleFunc("GET /api/backend", a.backend)
	mux.HandleFunc("POST /api/setup", a.setup)
	mux.HandleFunc("GET /api/setup-info", a.setupInfo)
	root, _ := fs.Sub(web, "web")
	files := http.FileServer(http.FS(root))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/setup" {
			content, e := fs.ReadFile(root, "index.html")
			if e != nil {
				http.Error(w, "App assets missing", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(content)
			return
		}
		files.ServeHTTP(w, r)
	})
	var handler http.Handler = mux
	if a.base != "" {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == a.base || strings.HasPrefix(r.URL.Path, a.base+"/") {
				http.StripPrefix(a.base, mux).ServeHTTP(w, r)
			} else {
				mux.ServeHTTP(w, r)
			}
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originURL, _ := url.Parse(a.cfg.Origin)
		localAlt := strings.Replace(a.listen, "127.0.0.1:", "localhost:", 1)
		if r.Host != originURL.Host && r.Host != a.listen && r.Host != localAlt {
			http.Error(w, "Unknown host", 403)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'self'")
		if r.Method != "GET" && r.Method != "HEAD" {
			expected := a.cfg.Origin
			if origin := r.Header.Get("Origin"); origin != "" && origin != expected {
				http.Error(w, "Origin rejected", 403)
				return
			}
		}
		if a.demo && strings.HasPrefix(strings.TrimPrefix(r.URL.Path, a.base), "/api/") {
			a.demoSession(w, r)
		}
		handler.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	json.NewEncoder(w).Encode(map[string]string{"error": e.Error()})
}
func (a *App) preferences(w http.ResponseWriter, r *http.Request) {
	s, ok := a.user(w, r)
	if !ok {
		return
	}
	sum := sha256.Sum256([]byte(a.cfg.Issuer + "\x00" + s.Subject))
	key := hex.EncodeToString(sum[:])
	p := filepath.Join(a.data, "preferences", key+".json")
	if r.Method == "GET" {
		var prefs map[string]any
		if secure.LoadJSON(p, &prefs) != nil {
			prefs = map[string]any{}
		}
		writeJSON(w, prefs)
		return
	}
	var prefs map[string]any
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&prefs) != nil {
		http.Error(w, "Invalid preferences", 400)
		return
	}
	if e := secure.SaveJSON(p, prefs); e != nil {
		fail(w, e)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
func (a *App) backend(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.user(w, r); !ok {
		return
	}
	c, s, e := shared.Load(a.root)
	if a.demo {
		writeJSON(w, map[string]any{"demo": true})
		return
	}
	if e != nil {
		fail(w, errors.New("Shared service needs setup"))
		return
	}
	run, _ := shared.ReadRunning(a.root)
	writeJSON(w, map[string]any{"version": c.Version, "apps": s.Apps, "ready": run.Ready, "instance": run.Instance, "restarts": run.Restarts, "externalInstancesDetected": len(s.External)})
}
func (a *App) demoSession(w http.ResponseWriter, r *http.Request) {
	const id = "demo-local"
	a.mu.Lock()
	if a.sessions[id] == nil {
		a.sessions[id] = &Session{Subject: "demo", Name: "Alex Morgan", Email: "alex@example.test", CSRF: "demo-local-csrf", Expires: time.Now().Add(24 * time.Hour)}
	}
	a.mu.Unlock()
	r.AddCookie(&http.Cookie{Name: a.kind + "_session", Value: id})
	if r.Method != "GET" && r.Header.Get("X-CSRF-Token") == "" {
		r.Header.Set("X-CSRF-Token", "demo-local-csrf")
	}
}
func (a *App) setup(w http.ResponseWriter, r *http.Request) {
	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	if net.ParseIP(peer) == nil || !net.ParseIP(peer).IsLoopback() || r.Header.Get("X-Setup-Token") != a.setupToken || r.Header.Get("X-Forwarded-For") != "" {
		http.Error(w, "Use the private setup link on this computer", 403)
		return
	}
	var c shared.Config
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&c) != nil {
		http.Error(w, "Invalid setup", 400)
		return
	}
	if c.Domain == "" {
		c.Domain = "workspace.test"
	}
	if c.Version == "" {
		c.Version = shared.EngineVersion
	}
	if c.EnginePort == 0 {
		c.EnginePort = 18997
	}
	if _, e := shared.Install(a.root, a.kind, Version, &c); e != nil {
		fail(w, e)
		return
	}
	exe, _ := os.Executable()
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if e := shared.Ensure(ctx, a.root, filepath.Join(filepath.Dir(exe), "WorkspaceServices.exe")); e != nil {
		fail(w, e)
		return
	}
	a.mu.Lock()
	a.cfg.Issuer = c.Issuer
	a.cfg.ClientID = c.ClientID
	a.cfg.ClientSecret = c.ClientSecret
	a.cfg.Domain = c.Domain
	a.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

// DAV URLs are never accepted as arbitrary proxy destinations.
func davPath(raw, prefix string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, prefix) || strings.ContainsAny(u.Path, "\\\x00") {
		return "", errors.New("invalid DAV resource")
	}
	for _, part := range strings.Split(u.Path, "/") {
		if part == ".." || part == "." {
			return "", errors.New("invalid DAV path")
		}
	}
	return u.EscapedPath(), nil
}

func (a *App) setupInfo(w http.ResponseWriter, r *http.Request) {
	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	if net.ParseIP(peer) == nil || !net.ParseIP(peer).IsLoopback() || r.Header.Get("X-Setup-Token") != a.setupToken {
		http.Error(w, "Private setup authorization required", 403)
		return
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	var manifest map[string]any
	_ = secure.LoadJSON(filepath.Join(dir, "engine.lock.json"), &manifest)
	hash, _ := manifest["engineSha256"].(string)
	writeJSON(w, map[string]any{"enginePath": filepath.Join(dir, "stalwart.exe"), "engineSha256": hash, "version": shared.EngineVersion, "issuer": os.Getenv("WORKSPACE_OIDC_ISSUER"), "clientId": os.Getenv("WORKSPACE_OIDC_CLIENT_ID"), "clientSecret": os.Getenv("WORKSPACE_OIDC_CLIENT_SECRET")})
}
