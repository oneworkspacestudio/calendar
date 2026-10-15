// SPDX-License-Identifier: AGPL-3.0-only
package shared

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/calendar/shared/secure"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const Schema = 1
const EngineVersion = "0.16.26"
const EngineArchiveSHA256 = "5f8370b259330e9eba65ed21bd59af92cfecd9597aea17594d803ec68b4bc291"

type Config struct {
	Issuer         string `json:"issuer"`
	ClientID       string `json:"clientId"`
	ClientSecret   string `json:"clientSecret"`
	Domain         string `json:"domain"`
	Hostname       string `json:"hostname"`
	EngineHostname string `json:"engineHostname"`
	EnginePort     int    `json:"enginePort"`
	EnginePath     string `json:"enginePath"`
	EngineSHA256   string `json:"engineSha256"`
	Version        string `json:"version"`
	Configured     bool   `json:"configured"`
}
type State struct {
	Schema        int               `json:"schema"`
	Root          string            `json:"root"`
	Apps          map[string]string `json:"apps"`
	EngineVersion string            `json:"engineVersion"`
	EngineSHA256  string            `json:"engineSha256"`
	Updated       time.Time         `json:"updated"`
	External      []string          `json:"externalInstances,omitempty"`
}
type Running struct {
	PID       int    `json:"pid"`
	EnginePID int    `json:"enginePid"`
	URL       string `json:"url"`
	Instance  string `json:"instance"`
	Ready     bool   `json:"ready"`
	Restarts  int    `json:"restarts"`
	Error     string `json:"error,omitempty"`
}

func DefaultRoot() string {
	if p := os.Getenv("WORKSPACE_SHARED_DATA"); p != "" {
		return p
	}
	if roots := DiscoverManaged(); len(roots) == 1 {
		return roots[0]
	}
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "OneWorkspace", "Shared")
}
func InstanceID(root string) string {
	abs, _ := filepath.Abs(root)
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(abs))))
	return hex.EncodeToString(sum[:12])
}
func ValidURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")))
}
func (c Config) Validate() error {
	if !ValidURL(c.Issuer) || c.ClientID == "" || len(c.ClientSecret) < 8 {
		return errors.New("MyAccounts issuer, shared client ID and secret are required")
	}
	if c.EnginePort < 1024 || c.EnginePort > 65535 {
		return errors.New("choose an unprivileged backend port")
	}
	if c.Version != EngineVersion {
		return errors.New("unsupported Stalwart version; use a tested engine package")
	}
	if _, e := hex.DecodeString(c.EngineSHA256); e != nil || len(c.EngineSHA256) != 64 {
		return errors.New("invalid engine SHA256")
	}
	if c.Domain == "" || strings.ContainsAny(c.Domain, "/\\\n\r @") || c.EnginePath == "" {
		return errors.New("invalid shared domain or engine path")
	}
	return nil
}
func Load(root string) (Config, State, error) {
	var c Config
	var s State
	if e := owned(root); e != nil {
		return c, s, e
	}
	e := secure.LoadSecret(filepath.Join(root, "settings.dpapi"), &c)
	if e != nil {
		return c, s, e
	}
	e = secure.LoadJSON(filepath.Join(root, "service.json"), &s)
	if e == nil && (s.Schema != Schema || InstanceID(s.Root) != InstanceID(root)) {
		e = errors.New("shared registry does not belong to this installation")
	}
	return c, s, e
}
func acquire(root string) (*os.File, error) {
	if e := Prepare(root); e != nil {
		return nil, e
	}
	return secure.Lock(filepath.Join(root, "install.lock"))
}

func validApp(app string) bool { return app == "calendar" || app == "mail" || app == "contacts" }

// Install registers another consumer. Existing authoritative configuration is immutable here.
func Install(root, app, appVersion string, in *Config) (Config, error) {
	if !validApp(app) || appVersion == "" {
		return Config{}, errors.New("unknown or unversioned dependent app")
	}
	lock, e := acquire(root)
	if e != nil {
		return Config{}, e
	}
	defer lock.Close()
	c, s, e := Load(root)
	if e != nil && !os.IsNotExist(e) {
		return c, e
	}
	if os.IsNotExist(e) {
		if in == nil {
			return c, errors.New("configure the shared service once before installing this app")
		}
		c = *in
		if e = c.Validate(); e != nil {
			return c, e
		}
		if e = VerifyFile(c.EnginePath, c.EngineSHA256); e != nil {
			return c, e
		}
		dest := filepath.Join(root, "bin", "stalwart-"+c.Version+".exe")
		if e = secure.CopyFile(c.EnginePath, dest); e != nil {
			return c, e
		}
		c.EnginePath = dest
		c.Configured = false
		if c.Hostname == "" {
			c.Hostname = "workspace.local"
		}
		if c.EngineHostname == "" {
			c.EngineHostname = c.Hostname
		}
		if e = secure.SaveSecret(filepath.Join(root, "settings.dpapi"), c); e != nil {
			return c, e
		}
		s = State{Schema: Schema, Root: root, Apps: map[string]string{}, EngineVersion: c.Version, EngineSHA256: c.EngineSHA256, External: DiscoverExternal("")}
	} else if in != nil && (in.Issuer != c.Issuer || in.ClientID != c.ClientID || in.Domain != c.Domain || in.EnginePort != c.EnginePort || in.EngineSHA256 != c.EngineSHA256) {
		return c, errors.New("this installation already has a backend; adding an app cannot change identity, listeners, version or data")
	}
	if s.Apps == nil {
		s.Apps = map[string]string{}
	}
	s.Apps[app] = appVersion
	s.Updated = time.Now().UTC()
	if e = secure.SaveJSON(filepath.Join(root, "service.json"), s); e != nil {
		return c, e
	}
	return c, RegisterInstallation(root)
}

// Uninstall only unregisters an app. No stop, binary, calendar, attachment or store deletion.
func Uninstall(root, app string) error {
	if !validApp(app) {
		return errors.New("unknown app")
	}
	lock, e := acquire(root)
	if e != nil {
		return e
	}
	defer lock.Close()
	_, s, e := Load(root)
	if e != nil {
		return e
	}
	delete(s.Apps, app)
	s.Updated = time.Now().UTC()
	return secure.SaveJSON(filepath.Join(root, "service.json"), s)
}
func VerifyFile(path, expected string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return e
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), expected) {
		return errors.New("engine package checksum mismatch")
	}
	return nil
}
func EngineURL(c Config) string { return fmt.Sprintf("http://127.0.0.1:%d", c.EnginePort) }
func ReadRunning(root string) (Running, error) {
	var r Running
	e := secure.LoadJSON(filepath.Join(root, "running.json"), &r)
	return r, e
}
func Healthy(ctx context.Context, root string) bool {
	run, e := ReadRunning(root)
	if e != nil || run.Instance != InstanceID(root) || !strings.HasPrefix(run.URL, "http://127.0.0.1:") {
		return false
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", run.URL+"/healthz", nil)
	client := http.Client{Timeout: 2 * time.Second}
	resp, e := client.Do(req)
	if e != nil {
		return false
	}
	defer resp.Body.Close()
	var out Running
	return resp.StatusCode == 200 && json.NewDecoder(resp.Body).Decode(&out) == nil && out.Instance == run.Instance && out.Ready
}
func Ensure(ctx context.Context, root, manager string) error {
	if Healthy(ctx, root) {
		return nil
	}
	if manager == "" {
		return errors.New("WorkspaceServices.exe is missing; install the shared service package")
	}
	cmd := exec.Command(manager, "run", "--root", root)
	cmd.Dir = filepath.Dir(manager)
	Hide(cmd)
	if e := cmd.Start(); e != nil {
		return e
	}
	go cmd.Wait()
	for {
		if Healthy(ctx, root) {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("shared Stalwart did not become ready; inspect shared service status/logs")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// Run owns the lifetime lock independently of app registration and app windows.
func Run(ctx context.Context, root string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, _, e := Load(root)
	if e != nil {
		return e
	}
	if e = c.Validate(); e != nil {
		return e
	}
	mutex, e := ServiceMutex(root)
	if e != nil {
		return e
	}
	defer mutex()
	lock, e := secure.Lock(filepath.Join(root, "runtime.lock"))
	if e != nil {
		return errors.New("the shared Stalwart service already has an owner")
	}
	defer lock.Close()
	if e = CheckRegistration(root); e != nil {
		return e
	}
	engine, e := newNative(root, c)
	if e != nil {
		return e
	}
	defer engine.job.Close()
	defer engine.stopEngine()
	run := Running{PID: os.Getpid(), Instance: InstanceID(root)}
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return e
	}
	run.URL = "http://" + listener.Addr().String()
	mux := http.NewServeMux()
	var stateMu = make(chan struct{}, 1)
	stateMu <- struct{}{}
	save := func() { _ = secure.SaveJSON(filepath.Join(root, "running.json"), run) }
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		<-stateMu
		defer func() { stateMu <- struct{}{} }()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if !run.Ready {
			w.WriteHeader(503)
		}
		json.NewEncoder(w).Encode(run)
	})
	controlToken := secure.Token()
	if e = secure.SaveSecret(filepath.Join(root, "control.dpapi"), map[string]string{"token": controlToken}); e != nil {
		return e
	}
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) {
		peer, _, _ := net.SplitHostPort(r.RemoteAddr)
		host := net.ParseIP(peer)
		if host == nil || !host.IsLoopback() || r.Host != listener.Addr().String() || r.Header.Get("Origin") != "" || r.Header.Get("X-Forwarded-For") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+controlToken)) != 1 {
			http.Error(w, "Local service owner authorization required", 403)
			return
		}
		w.WriteHeader(202)
		cancel()
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	save()
	if !c.Configured {
		if e = engine.initialize(ctx); e != nil {
			<-stateMu
			run.Error = "Shared backend setup failed; existing data retained"
			save()
			stateMu <- struct{}{}
			return fmt.Errorf("bootstrap shared Stalwart: %w", e)
		}
		c = engine.cfg
	} else {
		if e = engine.startEngine(); e != nil {
			return e
		}
	}
	if e = waitNative(ctx, engine); e != nil {
		return e
	}
	<-stateMu
	run.Ready = true
	run.EnginePID = engine.engine.Process.Pid
	save()
	stateMu <- struct{}{}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			<-stateMu
			run.Ready = false
			save()
			stateMu <- struct{}{}
			return nil
		case <-ticker.C:
			if nativeHealthy(ctx, engine) {
				failures = 0
				continue
			}
			failures++
			if failures < 3 {
				continue
			}
			<-stateMu
			run.Ready = false
			run.Restarts++
			run.Error = "Recovering shared backend"
			save()
			stateMu <- struct{}{}
			engine.stopEngine()
			if e = engine.startEngine(); e == nil {
				e = waitNative(ctx, engine)
			}
			if e != nil {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(time.Duration(min(failures, 10)) * time.Second):
				}
				continue
			}
			failures = 0
			<-stateMu
			run.Ready = true
			run.EnginePID = engine.engine.Process.Pid
			run.Error = ""
			save()
			stateMu <- struct{}{}
		}
	}
}
func nativeHealthy(ctx context.Context, a *nativeEngine) bool {
	var recovery map[string]string
	if secure.LoadSecret(filepath.Join(a.data, "recovery.dpapi"), &recovery) != nil {
		return false
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", EngineURL(a.cfg)+"/jmap/session", nil)
	req.SetBasicAuth("admin", recovery["password"])
	resp, e := a.client.Do(req)
	if e != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}
func waitNative(ctx context.Context, a *nativeEngine) error {
	deadline := time.NewTimer(60 * time.Second)
	defer deadline.Stop()
	for {
		if nativeHealthy(ctx, a) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("Stalwart health check timed out")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// Upgrade requires an explicit maintenance operation with all app processes stopped by the installer.
// It retains prior engine/settings and copies stopped persistent data before switching.
func Upgrade(root string, next Config) error {
	lock, e := acquire(root)
	if e != nil {
		return e
	}
	defer lock.Close()
	runtime, e := secure.Lock(filepath.Join(root, "runtime.lock"))
	if e != nil {
		return errors.New("stop the shared service for the coordinated upgrade; individual app installs cannot upgrade it")
	}
	defer runtime.Close()
	old, s, e := Load(root)
	if e != nil {
		return e
	}
	if e = next.Validate(); e != nil {
		return e
	}
	if next.Issuer != old.Issuer || next.ClientID != old.ClientID || next.Domain != old.Domain || next.EnginePort != old.EnginePort {
		return errors.New("upgrade cannot change identity or listeners")
	}
	if e = VerifyFile(next.EnginePath, next.EngineSHA256); e != nil {
		return e
	}
	backup := filepath.Join(root, "backups", time.Now().UTC().Format("20060102T150405.000000000"))
	for _, name := range []string{"ownership.json", "store", "engine.json", "settings.dpapi", "recovery.dpapi", "service.json"} {
		if e = copyTree(filepath.Join(root, name), filepath.Join(backup, name)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	dest := filepath.Join(root, "bin", "stalwart-"+next.Version+"-"+next.EngineSHA256[:12]+".exe")
	if e = secure.CopyFile(next.EnginePath, dest); e != nil {
		return e
	}
	old.EnginePath = dest
	old.EngineSHA256 = next.EngineSHA256
	old.Version = next.Version
	s.EngineSHA256 = old.EngineSHA256
	s.EngineVersion = old.Version
	if e = secure.SaveSecret(filepath.Join(root, "settings.dpapi"), old); e != nil {
		return e
	}
	return secure.SaveJSON(filepath.Join(root, "service.json"), s)
}
func copyTree(src, dst string) error {
	info, e := os.Lstat(src)
	if e != nil {
		return e
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("backup refuses linked private data")
	}
	if !info.IsDir() {
		return secure.CopyFile(src, dst)
	}
	if e = os.MkdirAll(dst, 0700); e != nil {
		return e
	}
	entries, e := os.ReadDir(src)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if e = copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); e != nil {
			return e
		}
	}
	return nil
}

// Stop is an explicit owner operation; uninstall never calls it.
func Stop(ctx context.Context, root string) error {
	run, e := ReadRunning(root)
	if e != nil {
		return e
	}
	var secret map[string]string
	if e = secure.LoadSecret(filepath.Join(root, "control.dpapi"), &secret); e != nil {
		return e
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", run.URL+"/stop", nil)
	req.Header.Set("Authorization", "Bearer "+secret["token"])
	client := http.Client{Timeout: 5 * time.Second}
	res, e := client.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode != 202 {
		return errors.New("shared service rejected stop")
	}
	deadline := time.NewTimer(12 * time.Second)
	defer deadline.Stop()
	for {
		lock, e := secure.Lock(filepath.Join(root, "runtime.lock"))
		if e == nil {
			lock.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("service is still stopping; retry maintenance after it exits")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Backup copies a stopped store and its protected settings; no source files are removed.
func Backup(root, destination string) error {
	runtime, e := secure.Lock(filepath.Join(root, "runtime.lock"))
	if e != nil {
		return errors.New("stop the shared service for a consistent backup")
	}
	defer runtime.Close()
	abs, e := filepath.Abs(destination)
	if e != nil {
		return e
	}
	source, _ := filepath.Abs(root)
	if strings.HasPrefix(strings.ToLower(abs+string(os.PathSeparator)), strings.ToLower(source+string(os.PathSeparator))) {
		return errors.New("choose a backup destination outside the shared data folder")
	}
	if _, e = os.Stat(destination); !os.IsNotExist(e) {
		return errors.New("backup destination must be a new folder")
	}
	if e = os.MkdirAll(destination, 0700); e != nil {
		return e
	}
	if e = secure.ProtectDir(destination); e != nil {
		return e
	}
	for _, name := range []string{"ownership.json", "store", "engine.json", "settings.dpapi", "recovery.dpapi", "service.json", "apps"} {
		if e = copyTree(filepath.Join(root, name), filepath.Join(destination, name)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
