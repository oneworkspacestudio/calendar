package shared

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/oneworkspacestudio/calendar/shared/secure"
	"path/filepath"
	"time"
)

type MailListeners struct {
	SMTP        int  `json:"smtpPort"`
	IMAP        int  `json:"imapPort"`
	PublicSMTP  int  `json:"publicSmtpPort"`
	AllowPublic bool `json:"allowPublicSmtp"`
}

func (m MailListeners) Validate() error {
	if m.PublicSMTP != 0 && !m.AllowPublic {
		return errors.New("public SMTP requires explicit --allow-public-smtp consent")
	}
	seen := map[int]bool{}
	for _, p := range []int{m.SMTP, m.IMAP, m.PublicSMTP} {
		if p < 0 || p > 65535 || (p > 0 && seen[p]) {
			return errors.New("mail listener ports must be distinct and valid")
		}
		if p > 0 {
			seen[p] = true
		}
	}
	return nil
}

// ConfigureMailListeners is an explicit installation-owner maintenance operation.
// It only edits our three named mail listeners. Application installation never calls it.
func ConfigureMailListeners(ctx context.Context, root string, m MailListeners) error {
	if e := m.Validate(); e != nil {
		return e
	}
	lock, e := acquire(root)
	if e != nil {
		return e
	}
	defer lock.Close()
	runtime, e := secure.Lock(filepath.Join(root, "runtime.lock"))
	if e != nil {
		return errors.New("stop shared service before changing listeners")
	}
	defer runtime.Close()
	c, _, e := Load(root)
	if e != nil {
		return e
	}
	if !c.Configured {
		return errors.New("complete shared service setup first")
	}
	if m.SMTP == c.EnginePort || m.IMAP == c.EnginePort || m.PublicSMTP == c.EnginePort {
		return errors.New("mail listener conflicts with private HTTP")
	}
	backup := filepath.Join(root, "backups", "listeners-"+time.Now().UTC().Format("20060102T150405.000000000"))
	for _, name := range []string{"ownership.json", "store", "engine.json", "settings.dpapi", "recovery.dpapi", "service.json"} {
		if e = copyTree(filepath.Join(root, name), filepath.Join(backup, name)); e != nil {
			return e
		}
	}
	a, e := newNative(root, c)
	if e != nil {
		return e
	}
	defer a.job.Close()
	defer a.stopEngine()
	if e = a.startEngineMode(true); e != nil {
		return e
	}
	if e = waitNative(ctx, a); e != nil {
		return e
	}
	auth, e := recoveryAuth(root)
	if e != nil {
		return e
	}
	existing, e := a.jmap(ctx, auth, "x:NetworkListener/get", map[string]any{})
	if e != nil {
		return e
	}
	desired := map[string]any{}
	for _, item := range []struct {
		name, protocol string
		port           int
		public         bool
	}{{"workspace-mail-smtp", "smtp", m.SMTP, false}, {"workspace-mail-imap", "imap", m.IMAP, false}, {"workspace-public-smtp", "smtp", m.PublicSMTP, true}} {
		if item.port == 0 {
			continue
		}
		host := "127.0.0.1"
		if item.public {
			host = "0.0.0.0"
		}
		desired[item.name] = map[string]any{"name": item.name, "protocol": item.protocol, "bind": map[string]bool{fmt.Sprintf("%s:%d", host, item.port): true}, "useTls": item.public, "tlsImplicit": false}
	}
	updates := map[string]any{}
	destroy := []string{}
	list, _ := existing["list"].([]any)
	for _, item := range list {
		entry, _ := item.(map[string]any)
		name, _ := entry["name"].(string)
		id, _ := entry["id"].(string)
		if name != "workspace-mail-smtp" && name != "workspace-mail-imap" && name != "workspace-public-smtp" {
			continue
		}
		if next, ok := desired[name]; ok {
			updates[id] = next
			delete(desired, name)
		} else {
			destroy = append(destroy, id)
		}
	}
	_, e = a.jmap(ctx, auth, "x:NetworkListener/set", map[string]any{"create": desired, "update": updates, "destroy": destroy})
	return e
}
func recoveryAuth(root string) (string, error) {
	var secret map[string]string
	if e := secure.LoadSecret(filepath.Join(root, "recovery.dpapi"), &secret); e != nil {
		return "", e
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:"+secret["password"])), nil
}
func ListenerSnapshot(ctx context.Context, root string) (map[string]any, error) {
	c, _, e := Load(root)
	if e != nil {
		return nil, e
	}
	a, e := newNative(root, c)
	if e != nil {
		return nil, e
	}
	defer a.job.Close()
	auth, e := recoveryAuth(root)
	if e != nil {
		return nil, e
	}
	return a.jmap(ctx, auth, "x:NetworkListener/get", map[string]any{})
}
