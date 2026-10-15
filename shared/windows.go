//go:build windows

package shared

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func Hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
func registryPath(root string) string {
	return `Software\OneWorkspace\Installations\` + InstanceID(root)
}
func RegisterInstallation(root string) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, registryPath(root), registry.QUERY_VALUE|registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if old, _, e := k.GetStringValue("SharedData"); e == nil && InstanceID(old) != InstanceID(root) {
		return errors.New("existing registry entry points to another installation")
	}
	abs, _ := filepath.Abs(root)
	if e = k.SetStringValue("ManagedBy", "OneWorkspace.SharedStalwart"); e != nil {
		return e
	}
	return k.SetStringValue("SharedData", abs)
}
func CheckRegistration(root string) error {
	k, e := registry.OpenKey(registry.CURRENT_USER, registryPath(root), registry.QUERY_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	saved, _, e := k.GetStringValue("SharedData")
	if e != nil || InstanceID(saved) != InstanceID(root) {
		return errors.New("shared service registry check failed")
	}
	return nil
}
func ServiceMutex(root string) (func(), error) {
	name, _ := windows.UTF16PtrFromString(`Local\OneWorkspace.Stalwart.` + InstanceID(root))
	h, e := windows.CreateMutex(nil, true, name)
	if e != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("another manager owns the service mutex: %w", e)
	}
	return func() { windows.ReleaseMutex(h); windows.CloseHandle(h) }, nil
}
func DiscoverExternal(own string) []string {
	out := []string{}
	snap, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e == nil {
		defer windows.CloseHandle(snap)
		entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
		for e = windows.Process32First(snap, &entry); e == nil; e = windows.Process32Next(snap, &entry) {
			name := windows.UTF16ToString(entry.ExeFile[:])
			if strings.HasPrefix(strings.ToLower(name), "stalwart") {
				h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
				path := name
				if e == nil {
					buf := make([]uint16, 32768)
					n := uint32(len(buf))
					if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) == nil {
						path = windows.UTF16ToString(buf[:n])
					}
					windows.CloseHandle(h)
				}
				if !strings.EqualFold(path, own) {
					out = append(out, fmt.Sprintf("process %d: %s (unmanaged; not adopted)", entry.ProcessID, path))
				}
			}
		}
	}
	k, e := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services`, registry.ENUMERATE_SUB_KEYS)
	if e == nil {
		defer k.Close()
		names, _ := k.ReadSubKeyNames(-1)
		for _, name := range names {
			if strings.Contains(strings.ToLower(name), "stalwart") {
				out = append(out, "Windows service "+name+" (external; not adopted)")
			}
		}
	}
	return out
}
func Autostart(root, manager string, on bool) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	name := "OneWorkspaceServices-" + InstanceID(root)
	if !on {
		e = k.DeleteValue(name)
		if e == registry.ErrNotExist {
			return nil
		}
		return e
	}
	if strings.ContainsAny(root+manager, "\"\r\n") {
		return errors.New("invalid startup path")
	}
	if _, e = os.Stat(manager); e != nil {
		return e
	}
	return k.SetStringValue(name, `"`+manager+`" run --root "`+root+`"`)
}

// DiscoverManaged only returns our registered, readable installation markers.
// External service/process detection is separate and never makes an instance managed.
func DiscoverManaged() []string {
	key, e := registry.OpenKey(registry.CURRENT_USER, `Software\OneWorkspace\Installations`, registry.ENUMERATE_SUB_KEYS)
	if e != nil {
		return nil
	}
	defer key.Close()
	names, _ := key.ReadSubKeyNames(-1)
	roots := []string{}
	for _, name := range names {
		entry, e := registry.OpenKey(key, name, registry.QUERY_VALUE)
		if e != nil {
			continue
		}
		owner, _, _ := entry.GetStringValue("ManagedBy")
		root, _, _ := entry.GetStringValue("SharedData")
		entry.Close()
		if owner != "OneWorkspace.SharedStalwart" {
			continue
		}
		if _, _, e = Load(root); e == nil && InstanceID(root) == name {
			roots = append(roots, root)
		}
	}
	return roots
}
func PreparationMutex(root string) (func(), error) {
	name, _ := windows.UTF16PtrFromString(`Local\OneWorkspace.Prepare.` + InstanceID(root))
	handle, e := windows.CreateMutex(nil, false, name)
	if e != nil && e != windows.ERROR_ALREADY_EXISTS {
		return nil, e
	}
	result, e := windows.WaitForSingleObject(handle, 10000)
	if e != nil || result != windows.WAIT_OBJECT_0 && result != windows.WAIT_ABANDONED {
		windows.CloseHandle(handle)
		return nil, errors.New("another installer is preparing the shared folder")
	}
	return func() { windows.ReleaseMutex(handle); windows.CloseHandle(handle) }, nil
}
