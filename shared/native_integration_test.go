package shared_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-jose/go-jose/v4"
	"github.com/oneworkspacestudio/calendar/shared"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeAllAppOrdersIsolationRestartAndUninstall(t *testing.T) {
	exe := os.Getenv("WORKSPACE_TEST_STALWART")
	if exe == "" {
		t.Skip("set WORKSPACE_TEST_STALWART to the verified official executable for native acceptance")
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "fixture"))
	if e != nil {
		t.Fatal(e)
	}
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			json.NewEncoder(w).Encode(map[string]any{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "userinfo_endpoint": issuer.URL + "/userinfo", "jwks_uri": issuer.URL + "/jwks", "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/jwks":
			json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "fixture", Algorithm: "RS256", Use: "sig"}}})
		case "/userinfo":
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				http.Error(w, "unauthorized", 401)
				return
			}
			raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
			w.Write(raw)
		default:
			http.NotFound(w, r)
		}
	}))
	defer issuer.Close()
	makeToken := func(subject string) string {
		raw, _ := json.Marshal(map[string]any{"iss": issuer.URL, "sub": subject, "aud": "workspace-shared-test", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "name": "Synthetic User", "email": "contact@example.test"})
		sig, _ := signer.Sign(raw)
		token, _ := sig.CompactSerialize()
		return token
	}
	orders := [][]string{{"calendar", "mail", "contacts"}, {"calendar", "contacts", "mail"}, {"mail", "calendar", "contacts"}, {"mail", "contacts", "calendar"}, {"contacts", "calendar", "mail"}, {"contacts", "mail", "calendar"}}
	for _, order := range orders {
		t.Run(strings.Join(order, "-"), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "shared")
			l, e := net.Listen("tcp4", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			port := l.Addr().(*net.TCPAddr).Port
			l.Close()
			cfg := shared.Config{Issuer: issuer.URL, ClientID: "workspace-shared-test", ClientSecret: "fixture-client-secret", Domain: "workspace.test", EnginePort: port, Version: shared.EngineVersion, EnginePath: exe, EngineSHA256: "84c9345bc6d1056b059c09122529848b6bb58955a116d4520b3502a42092056d"}
			if _, e = shared.Install(root, order[0], "test", &cfg); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- shared.Run(ctx, root); close(done) }()
			defer func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Log("manager exit", err)
					}
				case <-time.After(15 * time.Second):
					t.Error("manager did not stop")
				}
			}()
			deadline := time.Now().Add(100 * time.Second)
			for !shared.Healthy(context.Background(), root) {
				select {
				case err := <-done:
					t.Fatalf("manager failed: %v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("native service did not become ready")
				}
				time.Sleep(300 * time.Millisecond)
			}
			snapshot, err := shared.ListenerSnapshot(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			listeners, _ := snapshot["list"].([]any)
			if len(listeners) != 1 {
				t.Fatalf("unexpected default listener count %d", len(listeners))
			}
			if listeners[0].(map[string]any)["protocol"] != "http" {
				t.Fatal("a mail listener was enabled implicitly")
			}
			initial, _ := shared.ReadRunning(root)
			for _, app := range order[1:] {
				if _, e = shared.Install(root, app, "test", nil); e != nil {
					t.Fatal(e)
				}
				next, _ := shared.ReadRunning(root)
				if next.EnginePID != initial.EnginePID {
					t.Fatal("adding an app started another Stalwart")
				}
			}
			base := fmt.Sprintf("http://127.0.0.1:%d", port)
			request := func(method, path, token, body string, headers map[string]string) (int, string, string) {
				req, _ := http.NewRequest(method, base+path, strings.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+token)
				for k, v := range headers {
					req.Header.Set(k, v)
				}
				client := http.Client{Timeout: 20 * time.Second}
				res, e := client.Do(req)
				if e != nil {
					t.Fatal(e)
				}
				defer res.Body.Close()
				raw, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
				return res.StatusCode, string(raw), res.Header.Get("ETag")
			}
			alice := makeToken("11111111-1111-4111-8111-111111111111")
			bob := makeToken("22222222-2222-4222-8222-222222222222")
			for _, token := range []string{alice, bob} {
				status, body, _ := request("GET", "/jmap/session", token, "", nil)
				if status != 200 {
					t.Fatalf("MyAccounts token rejected: %d %s", status, body)
				}
			}
			path := "/dav/cal/11111111-1111-4111-8111-111111111111%40workspace.test/default/acceptance.ics"
			calendar := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//One Workspace//Acceptance//EN\r\nBEGIN:VEVENT\r\nUID:acceptance-event\r\nDTSTART:20261015T090000Z\r\nDTEND:20261015T100000Z\r\nSUMMARY:Retained shared event\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
			status, body, etag := request("PUT", path, alice, calendar, map[string]string{"Content-Type": "text/calendar", "If-None-Match": "*"})
			if status != 201 {
				t.Fatalf("CalDAV create: %d %s", status, body)
			}
			status, body, _ = request("GET", path, bob, "", nil)
			if status != 403 && status != 404 {
				t.Fatalf("cross-user calendar exposed: %d %s", status, body)
			}
			status, body, _ = request("GET", path, alice, "", nil)
			if status != 200 || !strings.Contains(body, "Retained shared event") {
				t.Fatal("calendar not readable by its owner")
			}
			if etag == "" {
				t.Log("PUT ETag unavailable; GET supplies the version")
			}
			cardPath := "/dav/card/11111111-1111-4111-8111-111111111111%40workspace.test/default/contact.vcf"
			status, body, _ = request("PUT", cardPath, alice, "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:acceptance-contact\r\nFN:Synthetic Contact\r\nEMAIL:friend@example.test\r\nEND:VCARD\r\n", map[string]string{"Content-Type": "text/vcard", "If-None-Match": "*"})
			if status != 201 {
				t.Fatalf("CardDAV create: %d %s", status, body)
			}
			status, body, _ = request("GET", cardPath, bob, "", nil)
			if status != 403 && status != 404 {
				t.Fatalf("cross-user contacts exposed: %d", status)
			}
			jmap := `{"using":["urn:ietf:params:jmap:core","urn:ietf:params:jmap:mail"],"methodCalls":[["Mailbox/get",{"accountId":"`
			status, session, _ := request("GET", "/jmap/session", alice, "", nil)
			var info struct {
				PrimaryAccounts map[string]string `json:"primaryAccounts"`
			}
			json.NewDecoder(bytes.NewBufferString(session)).Decode(&info)
			account := info.PrimaryAccounts["urn:ietf:params:jmap:mail"]
			status, body, _ = request("POST", "/jmap", alice, jmap+account+`"},"m"]]}`, map[string]string{"Content-Type": "application/json"})
			if status != 200 || !strings.Contains(body, "Mailbox/get") {
				t.Fatalf("Mail JMAP failed: %d %s", status, body)
			}
			status, body, _ = request("POST", "/jmap", bob, jmap+account+`"},"m"]]}`, map[string]string{"Content-Type": "application/json"})
			if status != 200 || strings.Contains(body, `"Mailbox/get"`) {
				t.Fatal("another user's mailbox data was not rejected")
			}
			for _, app := range order {
				if e = shared.Uninstall(root, app); e != nil {
					t.Fatal(e)
				}
				current, _ := shared.ReadRunning(root)
				if current.EnginePID != initial.EnginePID {
					t.Fatal("uninstall interrupted remaining apps")
				}
				status, body, _ = request("GET", path, alice, "", nil)
				if status != 200 || !strings.Contains(body, "Retained shared event") {
					t.Fatal("uninstall lost calendar")
				}
				status, _, _ = request("GET", cardPath, alice, "", nil)
				if status != 200 {
					t.Fatal("uninstall lost contacts")
				}
			}
			process, e := os.FindProcess(initial.EnginePID)
			if e != nil {
				t.Fatal(e)
			}
			process.Kill()
			deadline = time.Now().Add(50 * time.Second)
			recovered := false
			for time.Now().Before(deadline) {
				run, _ := shared.ReadRunning(root)
				if run.Ready && run.EnginePID != initial.EnginePID && run.Restarts > 0 {
					recovered = true
					break
				}
				time.Sleep(300 * time.Millisecond)
			}
			if !recovered {
				t.Fatal("service failed to recover after engine termination")
			}
			status, body, _ = request("GET", path, alice, "", nil)
			if status != 200 || !strings.Contains(body, "Retained shared event") {
				t.Fatal("recovery lost persisted event")
			}
			if order[0] == "calendar" && order[1] == "mail" {
				if err := shared.Stop(context.Background(), root); err != nil {
					t.Fatal(err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
				current, _, err := shared.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if err = shared.Backup(root, filepath.Join(t.TempDir(), "snapshot")); err != nil {
					t.Fatal(err)
				}
				if err = shared.Upgrade(root, current); err != nil {
					t.Fatal(err)
				}
				ctx, cancel = context.WithCancel(context.Background())
				done = make(chan error, 1)
				go func() { done <- shared.Run(ctx, root); close(done) }()
				until := time.Now().Add(70 * time.Second)
				for !shared.Healthy(context.Background(), root) {
					if time.Now().After(until) {
						t.Fatal("planned restart after maintenance timed out")
					}
					time.Sleep(250 * time.Millisecond)
				}
				status, body, _ = request("GET", path, alice, "", nil)
				if status != 200 || !strings.Contains(body, "Retained shared event") {
					t.Fatal("planned restart or package reapply lost calendar")
				}
				t.Log("PASS explicit manager stop, offline backup, pinned-package upgrade/reapply and planned restart")
			}
			t.Log("PASS one backend, OIDC user isolation, CalDAV/CardDAV/JMAP, all removals and crash recovery")
		})
	}
}
