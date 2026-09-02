//go:build windows

package pathenv

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

// environmentKey is where per-user environment variables live.
const environmentKey = `Environment`

// Ensure prepends binDir to the user's PATH in the registry and broadcasts the
// change so newly started processes see it.
func Ensure(binDir string) (Status, error) {
	st := Status{OnPath: InPath(binDir)}

	key, err := registry.OpenKey(registry.CURRENT_USER, environmentKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return st, fmt.Errorf("open user environment registry key: %w", err)
	}
	defer key.Close()

	current, valueType, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return st, fmt.Errorf("read user PATH: %w", err)
	}
	if valueType == 0 {
		valueType = registry.EXPAND_SZ
	}

	if containsEntry(current, binDir) {
		st.Persisted = true
		st.NeedsRestart = !st.OnPath
		st.Detail = pathDetail(st)
		return st, nil
	}

	updated := binDir
	if current != "" {
		updated = binDir + ";" + strings.TrimSuffix(current, ";")
	}

	// EXPAND_SZ preserves any %VAR% references already in the user's PATH.
	if valueType == registry.EXPAND_SZ {
		err = key.SetExpandStringValue("Path", updated)
	} else {
		err = key.SetStringValue("Path", updated)
	}
	if err != nil {
		return st, fmt.Errorf("write user PATH: %w", err)
	}

	broadcastEnvironmentChange()

	st.Persisted = true
	st.NeedsRestart = !st.OnPath
	st.Detail = pathDetail(st)
	return st, nil
}

// Check reports the current state without modifying anything.
func Check(binDir string) Status {
	st := Status{OnPath: InPath(binDir)}
	if key, err := registry.OpenKey(registry.CURRENT_USER, environmentKey, registry.QUERY_VALUE); err == nil {
		defer key.Close()
		if current, _, err := key.GetStringValue("Path"); err == nil {
			st.Persisted = containsEntry(current, binDir)
		}
	}
	st.NeedsRestart = st.Persisted && !st.OnPath
	st.Detail = pathDetail(st)
	return st
}

func pathDetail(st Status) string {
	switch {
	case st.OnPath:
		return "on your PATH"
	case st.NeedsRestart:
		return "added to your PATH — open a new terminal to pick it up"
	default:
		return "not on your PATH"
	}
}

// platformFold case-folds a cleaned path; Windows paths are case-insensitive.
func platformFold(p string) string {
	return strings.ToLower(p)
}

func containsEntry(pathValue, dir string) bool {
	want := normalize(dir)
	for _, entry := range strings.Split(pathValue, ";") {
		if entry == "" {
			continue
		}
		if normalize(entry) == want {
			return true
		}
	}
	return false
}

// broadcastEnvironmentChange tells running shells that the environment block
// changed. Failure is non-fatal: a new terminal picks the change up anyway.
func broadcastEnvironmentChange() {
	const (
		hwndBroadcast   = 0xFFFF
		wmSettingChange = 0x001A
		smtoAbortIfHung = 0x0002
	)
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("SendMessageTimeoutW")

	param, err := syscall.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	var result uintptr
	proc.Call(
		uintptr(hwndBroadcast),
		uintptr(wmSettingChange),
		0,
		uintptr(unsafe.Pointer(param)),
		uintptr(smtoAbortIfHung),
		uintptr(5000),
		uintptr(unsafe.Pointer(&result)),
	)
}
