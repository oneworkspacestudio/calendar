package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/oneworkspacestudio/calendar/internal/testidp"
	"github.com/oneworkspacestudio/calendar/shared"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDAVPathsNeverBecomeAnOpenProxy(t *testing.T) {
	for _, bad := range []string{"https://evil.example/dav/cal/x", "//evil.example/dav/cal/x", "/jmap", "/dav/cal/../admin", "/dav/cal/%2e%2e/admin", "/dav/cal/a?admin=true", "/dav/cal/a#fragment", "/dav/cal/a\\..\\admin"} {
		if _, e := davPath(bad, "/dav/cal/"); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	if p, e := davPath("/dav/cal/alice%40example.test/default/test.ics", "/dav/cal/"); e != nil || p == "" {
		t.Fatal("valid DAV path rejected")
	}
}
func TestNativeApplicationsOIDCAndConditionalWrites(t *testing.T) {
	engine := os.Getenv("WORKSPACE_TEST_STALWART")
	if engine == "" {
		t.Skip("set WORKSPACE_TEST_STALWART for native application acceptance")
	}
	idp := testidp.New(t)
	root := filepath.Join(t.TempDir(), "shared")
	l, _ := net.Listen("tcp4", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cfg := shared.Config{Issuer: idp.Server.URL, ClientID: "shared-test", ClientSecret: "fixture-private-secret", Domain: "workspace.test", EnginePort: port, Version: shared.EngineVersion, EnginePath: engine, EngineSHA256: "84c9345bc6d1056b059c09122529848b6bb58955a116d4520b3502a42092056d"}
	if _, e := shared.Install(root, "calendar", "test", &cfg); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- shared.Run(ctx, root); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("service did not stop")
		}
	}()
	for until := time.Now().Add(80 * time.Second); !shared.Healthy(context.Background(), root); {
		select {
		case e := <-done:
			t.Fatalf("manager failed %v", e)
		default:
		}
		if time.Now().After(until) {
			t.Fatal("service timed out")
		}
		time.Sleep(250 * time.Millisecond)
	}
	run, _ := shared.ReadRunning(root)
	for _, kind := range []string{"calendar", "contacts"} {
		a := &App{kind: kind, root: root, data: filepath.Join(root, "apps", kind), sessions: map[string]*Session{}, pending: map[string]Login{}, client: &http.Client{Timeout: 20 * time.Second}, cfg: Config{Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Domain: cfg.Domain}}
		server := httptest.NewServer(a.Handler())
		defer server.Close()
		a.cfg.Origin = server.URL
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Jar: jar, Timeout: 25 * time.Second}
		res, e := client.Get(server.URL + "/auth/login")
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		res, e = client.Get(server.URL + "/api/session")
		if e != nil {
			t.Fatal(e)
		}
		var session map[string]any
		json.NewDecoder(res.Body).Decode(&session)
		res.Body.Close()
		if session["subject"] != idp.Subject {
			t.Fatalf("OIDC login failed: %v", session)
		}
		csrf, _ := session["csrf"].(string)
		if kind == "calendar" {
			res, e = client.Get(server.URL + "/api/calendars")
			if e != nil {
				t.Fatal(e)
			}
			var calendars []DAVItem
			raw, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if e = json.Unmarshal(raw, &calendars); e != nil || len(calendars) == 0 {
				t.Fatalf("CalDAV discovery: %d %s", res.StatusCode, raw)
			}
			item := DAVItem{Href: calendars[0].Href + "ui-acceptance.ics", ETag: "*", Data: "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//One Workspace//Test//EN\r\nBEGIN:VEVENT\r\nUID:ui-acceptance\r\nDTSTART:20261015T120000Z\r\nDTEND:20261015T130000Z\r\nSUMMARY:Calendar API write\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"}
			body, _ := json.Marshal(item)
			req, _ := http.NewRequest("PUT", server.URL+"/api/events", bytes.NewReader(body))
			req.Header.Set("X-CSRF-Token", csrf)
			req.Header.Set("Origin", server.URL)
			res, e = client.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			raw, _ = io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("Calendar API create: %d %s", res.StatusCode, raw)
			}
			req, _ = http.NewRequest("PUT", server.URL+"/api/events", bytes.NewReader(body))
			req.Header.Set("X-CSRF-Token", csrf)
			res, e = client.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			res.Body.Close()
			if res.StatusCode != 412 {
				t.Fatalf("Overwrite conflict was not preserved: %d", res.StatusCode)
			}
			req, _ = http.NewRequest("PUT", server.URL+"/api/events", bytes.NewReader(body))
			res, _ = client.Do(req)
			res.Body.Close()
			if res.StatusCode != 403 {
				t.Fatal("missing CSRF accepted")
			}
		} else {
			res, e = client.Get(server.URL + "/api/contacts")
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode != 200 {
				t.Fatalf("Contacts API read: %d %s", res.StatusCode, raw)
			}
		}
		if _, e = shared.Install(root, kind, "test", nil); e != nil {
			t.Fatal(e)
		}
	}
	mail := os.Getenv("WORKSPACE_TEST_MAIL")
	if mail != "" {
		l, _ := net.Listen("tcp4", "127.0.0.1:0")
		mailPort := l.Addr().(*net.TCPAddr).Port
		l.Close()
		cmd := exec.Command(mail, "--no-browser", "--shared-data", root, "--data", filepath.Join(root, "mail"), "--port", fmt.Sprint(mailPort))
		shared.Hide(cmd)
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		defer func() { cmd.Process.Kill(); cmd.Wait() }()
		base := fmt.Sprintf("http://localhost:%d", mailPort)
		ready := false
		for until := time.Now().Add(20 * time.Second); time.Now().Before(until); {
			res, e := http.Get(base + "/healthz")
			if e == nil {
				res.Body.Close()
				if res.StatusCode == 200 {
					ready = true
					break
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
		if !ready {
			t.Fatal("Mail.exe failed to start with shared Stalwart")
		}
		jar, _ := cookiejar.New(nil)
		client := http.Client{Jar: jar, Timeout: 20 * time.Second}
		res, e := client.Get(base + "/auth/login")
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		res, e = client.Get(base + "/api/session")
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		var info map[string]any
		json.Unmarshal(raw, &info)
		if info["subject"] != idp.Subject {
			t.Fatalf("Mail MyAccounts login failed: %d %s", res.StatusCode, raw)
		}
		req, _ := http.NewRequest("POST", base+"/api/jmap?session=1", bytes.NewBufferString("{}"))
		req.Header.Set("X-CSRF-Token", fmt.Sprint(info["csrf"]))
		req.Header.Set("Origin", base)
		res, e = client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		raw, _ = io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("Mail shared JMAP: %d %s", res.StatusCode, raw)
		}
		cmd.Process.Kill()
		current, _ := shared.ReadRunning(root)
		if current.EnginePID != run.EnginePID || !shared.Healthy(context.Background(), root) {
			t.Fatal("Closing Mail stopped shared backend")
		}
		t.Log("PASS Calendar and Contacts OIDC/API plus standalone Mail.exe OIDC/JMAP; closing Mail retains backend")
	}
}
