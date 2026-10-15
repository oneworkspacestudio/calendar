package shared

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/oneworkspacestudio/calendar/shared/secure"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"testing"
)

var orders = [][]string{{"calendar", "mail", "contacts"}, {"calendar", "contacts", "mail"}, {"mail", "calendar", "contacts"}, {"mail", "contacts", "calendar"}, {"contacts", "calendar", "mail"}, {"contacts", "mail", "calendar"}}

func fixture(t *testing.T) (string, Config) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "shared")
	engine := filepath.Join(t.TempDir(), "engine.exe")
	bytes := []byte("fixture executable, not launched")
	if e := os.WriteFile(engine, bytes, 0600); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(bytes)
	c := Config{Issuer: "http://127.0.0.1:16661", ClientID: "shared-test", ClientSecret: "private-test-secret", Domain: "workspace.test", Version: EngineVersion, EnginePort: 16662, EnginePath: engine, EngineSHA256: hex.EncodeToString(sum[:])}
	t.Cleanup(func() { registry.DeleteKey(registry.CURRENT_USER, registryPath(root)) })
	return root, c
}
func TestEveryInstallationAndRemovalOrderPreservesOneBackend(t *testing.T) {
	for _, order := range orders {
		t.Run(order[0]+"-"+order[1]+"-"+order[2], func(t *testing.T) {
			root, c := fixture(t)
			var path string
			for i, app := range order {
				var in *Config
				if i == 0 {
					in = &c
				}
				actual, e := Install(root, app, "1.0", in)
				if e != nil {
					t.Fatal(e)
				}
				if i == 0 {
					path = actual.EnginePath
					os.MkdirAll(filepath.Join(root, "store", "calendars"), 0700)
					os.WriteFile(filepath.Join(root, "store", "calendars", "retained.ics"), []byte("private event"), 0600)
				}
				if path != actual.EnginePath {
					t.Fatal("a second backend was allocated")
				}
				_, state, e := Load(root)
				if e != nil || len(state.Apps) != i+1 {
					t.Fatalf("registry mismatch: %v %+v", e, state)
				}
				if e = CheckRegistration(root); e != nil {
					t.Fatal(e)
				}
			}
			for _, app := range order {
				if e := Uninstall(root, app); e != nil {
					t.Fatal(e)
				}
				if body, e := os.ReadFile(filepath.Join(root, "store", "calendars", "retained.ics")); e != nil || string(body) != "private event" {
					t.Fatal("uninstall removed shared data")
				}
				if _, _, e := Load(root); e != nil {
					t.Fatal("uninstall removed backend configuration")
				}
			}
		})
	}
}
func TestRejectsReconfigureAndDuplicateServiceOwnership(t *testing.T) {
	root, c := fixture(t)
	if _, e := Install(root, "calendar", "1", &c); e != nil {
		t.Fatal(e)
	}
	c.Issuer = "http://127.0.0.1:18881"
	if _, e := Install(root, "mail", "1", &c); e == nil {
		t.Fatal("reconfiguration must require migration")
	}
	first, e := ServiceMutex(root)
	if e != nil {
		t.Fatal(e)
	}
	defer first()
	if second, e := ServiceMutex(root); e == nil {
		second()
		t.Fatal("duplicate service mutex accepted")
	}
	lock, e := secure.Lock(filepath.Join(root, "runtime.lock"))
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	if other, e := secure.Lock(filepath.Join(root, "runtime.lock")); e == nil {
		other.Close()
		t.Fatal("duplicate runtime lock accepted")
	}
}
func TestUpgradeBacksUpDataAndRequiresMaintenance(t *testing.T) {
	root, c := fixture(t)
	if _, e := Install(root, "calendar", "1", &c); e != nil {
		t.Fatal(e)
	}
	os.MkdirAll(filepath.Join(root, "store"), 0700)
	os.WriteFile(filepath.Join(root, "store", "retained"), []byte("calendar and contacts data"), 0600)
	lock, e := secure.Lock(filepath.Join(root, "runtime.lock"))
	if e != nil {
		t.Fatal(e)
	}
	if e = Upgrade(root, c); e == nil {
		t.Fatal("live upgrade must fail")
	}
	lock.Close()
	if e = Upgrade(root, c); e != nil {
		t.Fatal(e)
	}
	backups, _ := filepath.Glob(filepath.Join(root, "backups", "*", "store", "retained"))
	if len(backups) != 1 {
		t.Fatal("stopped data snapshot missing")
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != "calendar and contacts data" {
		t.Fatal("backup bytes mismatch")
	}
	c.Version = "99.0.0"
	if e = Upgrade(root, c); e == nil {
		t.Fatal("untested schema version accepted")
	}
}
func TestExternalDetectionDoesNotAdoptOrEdit(t *testing.T) {
	root, c := fixture(t)
	before := DiscoverExternal("")
	if _, e := Install(root, "contacts", "1", &c); e != nil {
		t.Fatal(e)
	}
	actual, _, e := Load(root)
	if e != nil {
		t.Fatal(e)
	}
	if !filepath.IsAbs(actual.EnginePath) || filepath.Dir(actual.EnginePath) != filepath.Join(root, "bin") {
		t.Fatal("external executable adopted")
	}
	_ = before
	if Healthy(context.Background(), root) {
		t.Fatal("unstarted backend reported healthy")
	}
}

func TestPublicMailRequiresExplicitConsent(t *testing.T) {
	if (MailListeners{PublicSMTP: 25}).Validate() == nil {
		t.Fatal("public SMTP enabled without consent")
	}
	if (MailListeners{SMTP: 2525, IMAP: 2143}).Validate() != nil {
		t.Fatal("private mail listeners rejected")
	}
	if (MailListeners{SMTP: 2525, IMAP: 2525}).Validate() == nil {
		t.Fatal("duplicate ports accepted")
	}
}

func TestUnmanagedDataIsNotAdoptedOrModified(t *testing.T) {
	root, c := fixture(t)
	if e := os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	foreign := filepath.Join(root, "config.toml")
	if e := os.WriteFile(foreign, []byte("external Stalwart settings"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := Install(root, "calendar", "1", &c); e == nil {
		t.Fatal("unmanaged directory adopted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 || entries[0].Name() != "config.toml" {
		t.Fatal("unmanaged directory was modified")
	}
	if b, _ := os.ReadFile(foreign); string(b) != "external Stalwart settings" {
		t.Fatal("external config changed")
	}
}
