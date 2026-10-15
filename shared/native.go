// SPDX-License-Identifier: AGPL-3.0-only
// Native integration derived from One Workspace Mail; see NOTICE.md.
package shared

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/calendar/shared/secure"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type nativeEngine struct {
	data       string
	cfg        Config
	mu         sync.RWMutex
	job        *secure.Job
	engine     *exec.Cmd
	held       *os.File
	engineDone chan struct{}
	restarting bool
	client     *http.Client
}

func newNative(root string, c Config) (*nativeEngine, error) {
	job, e := secure.NewJob()
	if e != nil {
		return nil, e
	}
	return &nativeEngine{data: root, cfg: c, job: job, client: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (a *nativeEngine) initialize(parent context.Context) error {
	c := a.cfg
	_, statErr := os.Stat(filepath.Join(a.data, "engine.json"))
	resume := statErr == nil
	credentials := map[string]string{}
	if e := secure.LoadSecret(filepath.Join(a.data, "recovery.dpapi"), &credentials); e != nil && !os.IsNotExist(e) {
		return e
	}
	password := credentials["password"]
	if password == "" {
		password = secure.Token()
	}
	if e := secure.SaveSecret(filepath.Join(a.data, "recovery.dpapi"), map[string]string{"password": password}); e != nil {
		return e
	}
	if e := secure.SaveSecret(filepath.Join(a.data, "pending.dpapi"), c); e != nil {
		return e
	}
	a.mu.Lock()
	a.cfg = c
	a.mu.Unlock()
	if e := a.startEngineMode(resume); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+password))
	var defaults map[string]any
	var e error
	if !resume {
		for n := 0; n < 60; n++ {
			defaults, e = a.jmap(ctx, auth, "x:Bootstrap/get", map[string]any{})
			if e == nil {
				break
			}
			select {
			case <-ctx.Done():
				return errors.New("Stalwart bootstrap timed out; inspect protected engine.log")
			case <-time.After(500 * time.Millisecond):
			}
		}
		if e != nil {
			return e
		}
		list, _ := defaults["list"].([]any)
		if len(list) == 0 {
			return errors.New("Bootstrap settings unavailable")
		}
		object := list[0].(map[string]any)
		id, _ := object["id"].(string)
		patch := map[string]any{"serverHostname": c.EngineHostname, "defaultDomain": c.Domain, "requestTlsCertificate": false, "generateDkimKeys": false, "dataStore": map[string]any{"@type": "RocksDb", "path": filepath.ToSlash(filepath.Join(a.data, "store"))}, "tracer": map[string]any{"@type": "Stdout", "enable": true}, "dnsServer": map[string]any{"@type": "Manual"}}
		if c.Issuer != "" {
			patch["directory"] = map[string]any{"@type": "Oidc", "description": "MyAccounts", "issuerUrl": c.Issuer, "requireAudience": c.ClientID, "requireScopes": map[string]bool{}, "claimUsername": "sub", "usernameDomain": c.Domain, "claimName": "name", "claimGroups": nil}
		}
		if _, e = a.jmap(ctx, auth, "x:Bootstrap/set", map[string]any{"update": map[string]any{id: patch}}); e != nil {
			return e
		}
		// Bootstrap writes the persistent registry; reopen it in recovery mode before
		// adding listeners, so no upstream public listeners are ever started.
		a.stopEngine()
		if e = a.startEngineMode(true); e != nil {
			return e
		}
	}
	for n := 0; n < 60; n++ {
		_, e = a.jmap(ctx, auth, "x:NetworkListener/get", map[string]any{})
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("Recovery startup timed out")
		case <-time.After(500 * time.Millisecond):
		}
	}
	if e != nil {
		return e
	}
	// Insert our own listeners before the first normal restart, suppressing upstream defaults.
	existing, e := a.jmap(ctx, auth, "x:NetworkListener/get", map[string]any{})
	if e != nil {
		return e
	}
	listeners := map[string]any{"http": map[string]any{"name": "workspace-http", "protocol": "http", "bind": map[string]bool{fmt.Sprintf("127.0.0.1:%d", c.EnginePort): true}, "useTls": false, "tlsImplicit": false}}
	updates := map[string]any{}
	if list, ok := existing["list"].([]any); ok {
		for _, item := range list {
			v := item.(map[string]any)
			id, _ := v["id"].(string)
			switch v["name"] {
			case "workspace-http":
				updates[id] = listeners["http"]
				delete(listeners, "http")
			default:
				return errors.New("Unexpected existing listener; review Stalwart before continuing")
			}
		}
	}
	_, e = a.jmap(ctx, auth, "x:NetworkListener/set", map[string]any{"create": listeners, "update": updates})
	if e != nil {
		return e
	}
	a.stopEngine()
	if e = a.startEngine(); e != nil {
		return e
	}
	if e = waitNative(ctx, a); e != nil {
		return e
	}
	if e = a.configureIdentity(ctx); e != nil {
		return e
	}

	if a.engine != nil {
		a.stopEngine()
	}
	c.Configured = true
	if e = secure.SaveSecret(filepath.Join(a.data, "settings.dpapi"), c); e != nil {
		return e
	}
	a.mu.Lock()
	a.cfg = c
	a.mu.Unlock()
	if e = a.startEngine(); e != nil {
		c.Configured = false
		_ = secure.SaveSecret(filepath.Join(a.data, "settings.dpapi"), c)
		a.mu.Lock()
		a.cfg = c
		a.mu.Unlock()
		return e
	}
	return nil
}
func (a *nativeEngine) startEngine() error { return a.startEngineMode(false) }
func (a *nativeEngine) stopEngine() {
	if a.engine == nil {
		return
	}
	a.mu.Lock()
	a.restarting = true
	a.mu.Unlock()
	_ = a.engine.Process.Kill()
	<-a.engineDone
	a.mu.Lock()
	a.restarting = false
	a.mu.Unlock()
	a.engine = nil
}
func (a *nativeEngine) startEngineMode(recoveryMode bool) error {
	path := a.cfg.EnginePath
	expected := a.cfg.EngineSHA256
	var e error
	a.held, e = secure.LockedBinary(path, expected)
	if e != nil {
		return e
	}
	var recovery map[string]string
	if e = secure.LoadSecret(filepath.Join(a.data, "recovery.dpapi"), &recovery); e != nil {
		a.held.Close()
		return e
	}
	c := a.cfg
	cmd := exec.Command(path, "--config", filepath.Join(a.data, "engine.json"))
	cmd.Dir = a.data
	engineHost := c.EngineHostname
	if engineHost == "" {
		engineHost = c.Hostname
	}
	cmd.Env = append(os.Environ(), "STALWART_RECOVERY_ADMIN=admin:"+recovery["password"], "STALWART_RECOVERY_MODE_PORT="+strconv.Itoa(c.EnginePort), "STALWART_HOSTNAME="+engineHost, "STALWART_PUBLIC_URL="+fmt.Sprintf("http://127.0.0.1:%d", c.EnginePort))
	cmd.Env = append(cmd.Env, "STALWART_RECOVERY_MODE="+strconv.FormatBool(recoveryMode))
	logFile, e := os.OpenFile(filepath.Join(a.data, "engine.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		a.held.Close()
		return e
	}
	filtered := &engineLog{out: logFile, secrets: []string{recovery["password"], c.ClientSecret}}
	cmd.Stdout = filtered
	cmd.Stderr = filtered
	if e = a.job.Start(cmd); e != nil {
		logFile.Close()
		a.held.Close()
		return e
	}
	a.engine = cmd
	held := a.held
	done := make(chan struct{})
	a.engineDone = done
	go func() {
		e := cmd.Wait()
		logFile.Close()
		held.Close()
		a.mu.RLock()
		intentional := a.restarting
		a.mu.RUnlock()
		close(done)
		if intentional {
			return
		}
		log.Printf("Stalwart stopped (%v); service manager will recover", e)
		return
	}()
	return nil
}

// The per-domain directory and the default Bearer-token directory are separate
// settings in Stalwart. Both must select this installation's MyAccounts client.
func (a *nativeEngine) configureIdentity(ctx context.Context) error {
	a.mu.RLock()
	c := a.cfg
	a.mu.RUnlock()
	if c.Issuer == "" {
		return nil
	}
	var credentials map[string]string
	if e := secure.LoadSecret(filepath.Join(a.data, "recovery.dpapi"), &credentials); e != nil {
		return e
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+credentials["password"]))
	var directories map[string]any
	var e error
	for n := 0; n < 60; n++ {
		directories, e = a.jmap(ctx, auth, "x:Directory/get", map[string]any{})
		if e == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if e != nil {
		return e
	}
	id := ""
	if list, ok := directories["list"].([]any); ok {
		for _, item := range list {
			v := item.(map[string]any)
			if v["issuerUrl"] == c.Issuer && v["requireAudience"] == c.ClientID && v["claimUsername"] == "sub" && v["usernameDomain"] == c.Domain {
				if id != "" {
					return errors.New("More than one matching MyAccounts directory exists; review Mail identity settings")
				}
				id, _ = v["id"].(string)
			}
		}
	}
	if id == "" {
		return errors.New("MyAccounts directory was not created")
	}
	settings, e := a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}})
	if e != nil {
		return e
	}
	list, _ := settings["list"].([]any)
	settingID := ""
	if len(list) == 1 {
		settingID, _ = list[0].(map[string]any)["id"].(string)
	} else {
		settingID, e = a.singletonID(ctx, auth)
		if e != nil {
			return e
		}
	}
	patch := map[string]any{"directoryId": id}
	// Restore only a missing ordinary-user role link, preserving custom policy.
	var roleLinks map[string]any
	if len(list) == 1 {
		roleLinks, _ = list[0].(map[string]any)["defaultUserRoleIds"].(map[string]any)
	}
	if len(roleLinks) == 0 {
		roles, err := a.jmap(ctx, auth, "x:Role/get", map[string]any{})
		if err != nil {
			return err
		}
		items, _ := roles["list"].([]any)
		userRole := ""
		for _, item := range items {
			role, ok := item.(map[string]any)
			if !ok || role["description"] != "User" || role["memberTenantId"] != nil {
				continue
			}
			enabled, _ := role["enabledPermissions"].(map[string]any)
			disabled, _ := role["disabledPermissions"].(map[string]any)
			inherited, _ := role["roleIds"].(map[string]any)
			if enabled["authenticate"] != true || enabled["jmapMailboxGet"] != true || len(inherited) != 0 || len(disabled) != 0 {
				continue
			}
			safe := true
			for permission := range enabled {
				if !ordinaryUserPermission(permission) {
					safe = false
				}
			}
			if !safe {
				continue
			}
			if userRole != "" {
				return errors.New("Multiple default User roles exist; the Workspace owner must review permissions")
			}
			userRole, _ = role["id"].(string)
		}
		if userRole == "" {
			return errors.New("The ordinary User role is missing; inspect the shared backend configuration")
		}
		patch["defaultUserRoleIds"] = map[string]bool{userRole: true}
	}
	_, e = a.jmap(ctx, auth, "x:Authentication/set", map[string]any{"update": map[string]any{settingID: patch}})
	if e == nil {
		check, err := a.jmap(ctx, auth, "x:Authentication/get", map[string]any{"ids": []string{"singleton"}})
		if err != nil {
			return err
		}
		items, _ := check["list"].([]any)
		if len(items) != 1 {
			return errors.New("Mail identity settings were not saved")
		}
		links, _ := items[0].(map[string]any)["defaultUserRoleIds"].(map[string]any)
		if len(links) == 0 {
			return errors.New("Mail's default user permissions are still missing")
		}
	}
	return e
}

// Matches the ordinary-user defaults in the pinned engine's permissions.rs.
// Some sys* permissions are scoped to the user's own settings and credentials.
func ordinaryUserPermission(p string) bool {
	if p == "authenticate" || p == "authenticateWithAlias" || p == "interactAi" {
		return true
	}
	for _, prefix := range []string{"jmap", "imap", "pop3", "calendar", "email", "dav", "sieve", "sysMaskedEmail", "sysArchivedItem", "sysAccountSettings", "sysPublicKey", "sysAccountPassword", "sysApiKey", "sysAppPassword"} {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return strings.HasPrefix(p, "sysSpamTrainingSample") && !strings.Contains(p, "Create")
}
func (a *nativeEngine) singletonID(ctx context.Context, auth string) (string, error) {
	// Stalwart's Id::singleton() (20080258862541) encodes as this literal.
	return "singleton", nil
}

func (a *nativeEngine) engineRequest(ctx context.Context, method, path, auth string, body io.Reader, contentType string) (*http.Response, error) {
	a.mu.RLock()
	port := a.cfg.EnginePort
	a.mu.RUnlock()
	req, e := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d", port)+path, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", auth)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return a.client.Do(req)
}
func (a *nativeEngine) jmap(ctx context.Context, auth, method string, args any) (map[string]any, error) {
	caps := []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:submission"}
	if strings.HasPrefix(method, "x:") {
		caps = append(caps, "urn:stalwart:jmap")
	}
	b, _ := json.Marshal(map[string]any{"using": caps, "methodCalls": []any{[]any{method, args, "m"}}})
	resp, e := a.engineRequest(ctx, "POST", "/jmap", auth, bytes.NewReader(b), "application/json")
	if e != nil {
		return nil, errors.New("Stalwart is not reachable; check engine.log")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode >= 500 {
			return nil, fmt.Errorf("Stalwart access rejected (HTTP %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("Stalwart rejected the request (HTTP %d)", resp.StatusCode)
	}
	return parseJMAPResponse(method, io.LimitReader(resp.Body, 16<<20))
}

// Submission can emit an implicit Email/set response for onSuccessUpdateEmail.
// Match our call, rather than treating the number of responses as success/failure.
func parseJMAPResponse(method string, body io.Reader) (map[string]any, error) {
	var out struct {
		MethodResponses [][]json.RawMessage `json:"methodResponses"`
	}
	if e := json.NewDecoder(body).Decode(&out); e != nil {
		return nil, unconfirmedJMAPResponse(method)
	}
	var primary []json.RawMessage
	var callErrors [][]json.RawMessage
	for _, call := range out.MethodResponses {
		if len(call) != 3 {
			return nil, unconfirmedJMAPResponse(method)
		}
		var name, id string
		if json.Unmarshal(call[0], &name) != nil || json.Unmarshal(call[2], &id) != nil {
			return nil, unconfirmedJMAPResponse(method)
		}
		if id == "m" && name == "error" {
			callErrors = append(callErrors, call)
		}
		if id == "m" && name == method {
			if primary != nil {
				return nil, unconfirmedJMAPResponse(method)
			}
			primary = call
		}
	}
	if primary == nil && len(callErrors) == 1 {
		primary = callErrors[0]
	}
	if primary == nil {
		return nil, unconfirmedJMAPResponse(method)
	}
	var name string
	_ = json.Unmarshal(primary[0], &name)
	var result map[string]any
	if json.Unmarshal(primary[1], &result) != nil || result == nil {
		return nil, unconfirmedJMAPResponse(method)
	}
	if name == "error" {
		return nil, fmt.Errorf("%s: %v (%v)", method, result["type"], result["description"])
	}
	for _, k := range []string{"notCreated", "notUpdated", "notDestroyed"} {
		if v, ok := result[k].(map[string]any); ok && len(v) > 0 {
			return nil, fmt.Errorf("%s: %v", method, v)
		}
	}
	if method == "EmailSubmission/set" {
		for _, call := range out.MethodResponses {
			var name string
			_ = json.Unmarshal(call[0], &name)
			if name != "Email/set" && name != "error" {
				continue
			}
			var effect map[string]any
			failed := json.Unmarshal(call[1], &effect) != nil || name == "error"
			for _, key := range []string{"notCreated", "notUpdated", "notDestroyed"} {
				if values, ok := effect[key].(map[string]any); ok && len(values) > 0 {
					failed = true
				}
			}
			if failed {
				result["workspaceWarnings"] = []string{"Message queued, but its Sent-folder update failed. Refresh Mail; do not send it again."}
			}
		}
	}
	return result, nil
}

func unconfirmedJMAPResponse(method string) error {
	if method == "EmailSubmission/set" {
		return errors.New("Mail could not confirm the send result. Check Sent and refresh before trying again; sending again could create a duplicate.")
	}
	return fmt.Errorf("Mail could not confirm the %s result. Refresh Mail and try again.", method)
}
