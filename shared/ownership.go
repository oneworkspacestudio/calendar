package shared

import (
	"encoding/json"
	"errors"
	"github.com/oneworkspacestudio/calendar/shared/secure"
	"os"
	"path/filepath"
)

type ownership struct {
	Product  string `json:"product"`
	Instance string `json:"instance"`
}

func owned(root string) error {
	raw, e := os.ReadFile(filepath.Join(root, "ownership.json"))
	if e != nil {
		return e
	}
	var marker ownership
	if json.Unmarshal(raw, &marker) != nil || marker.Product != "OneWorkspace.SharedStalwart" || marker.Instance != InstanceID(root) {
		return errors.New("shared ownership marker does not match this installation")
	}
	return nil
}

// Prepare never changes ACLs or writes into a nonempty unmanaged directory.
func Prepare(root string) error {
	unlock, e := PreparationMutex(root)
	if e != nil {
		return e
	}
	defer unlock()
	if e = owned(root); e == nil {
		return secure.ProtectDir(root)
	} else if !os.IsNotExist(e) {
		return e
	}
	entries, e := os.ReadDir(root)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if len(entries) != 0 {
		return errors.New("This folder contains unmanaged or legacy data. Explicit administrator migration is required; no data or configuration was adopted.")
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return e
	}
	if e = secure.ProtectDir(root); e != nil {
		return e
	}
	return secure.SaveJSON(filepath.Join(root, "ownership.json"), ownership{Product: "OneWorkspace.SharedStalwart", Instance: InstanceID(root)})
}
