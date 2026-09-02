package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/playdead/koa/internal/runner"
	"github.com/playdead/koa/internal/store"
)

// Launch starts an installed binary and streams its output into the in-app log
// panel (PRD §11). When auto-update is on for the app, koa updates it first.
func (s *Service) Launch(owner, name string) (Process, error) {
	tracked, ok := s.store.App(owner, name)
	if !ok {
		return Process{}, fmt.Errorf("%s/%s is not installed", owner, name)
	}

	if tracked.AutoUpdate {
		if _, err := s.CheckForUpdates(owner, name); err != nil {
			s.emit(EventToast, Toast{Kind: "warning", Message: fmt.Sprintf("Could not check %s for updates: %s", name, err)})
		}
		if refreshed, ok := s.store.App(owner, name); ok {
			tracked = refreshed
		}
	}

	binary := tracked.BinaryPath
	if binary == "" {
		binary = s.paths.BinaryPath(name)
	}
	if _, err := os.Stat(binary); err != nil {
		return Process{}, fmt.Errorf("%s is missing from your koa bin folder — reinstall it", name)
	}

	proc, err := s.procs.Launch(runner.Spec{
		Owner:      owner,
		Repo:       name,
		BinaryPath: binary,
		Dir:        homeOrEmpty(),
		Env:        os.Environ(),
	})
	if err != nil {
		return Process{}, err
	}

	s.emit(EventApps, s.Installed())
	return proc, nil
}

// homeOrEmpty gives launched apps a sensible working directory rather than
// koa's own, which may be anywhere.
func homeOrEmpty() string {
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// ListProcesses returns every process tab, live or finished (PRD §11).
func (s *Service) ListProcesses() []Process {
	out := s.procs.List()
	if out == nil {
		return []Process{}
	}
	return out
}

// ProcessLogs returns a process's retained output.
func (s *Service) ProcessLogs(id string) ([]LogLine, error) {
	lines, err := s.procs.Logs(id)
	if err != nil {
		if errors.Is(err, runner.ErrNotFound) {
			return []LogLine{}, nil
		}
		return nil, err
	}
	if lines == nil {
		return []LogLine{}, nil
	}
	return lines, nil
}

// StopProcess terminates a running process (PRD §11, the Stop action).
func (s *Service) StopProcess(id string) error {
	if err := s.procs.Stop(id); err != nil {
		return err
	}
	s.emit(EventApps, s.Installed())
	return nil
}

// ClearLogs empties one log panel without stopping the process.
func (s *Service) ClearLogs(id string) error { return s.procs.Clear(id) }

// CloseProcess stops the process if needed and removes its tab.
func (s *Service) CloseProcess(id string) error {
	if err := s.procs.Forget(id); err != nil {
		return err
	}
	s.emit(EventApps, s.Installed())
	return nil
}

// RunningCount drives the pulsing dot on the Running nav item (PRD §5.2).
func (s *Service) RunningCount() int { return s.procs.RunningCount() }

// runningKeys is a helper for views that need to know which apps are live.
func (s *Service) runningKeys() map[string]bool {
	out := map[string]bool{}
	for _, p := range s.procs.List() {
		if p.Running {
			out[store.Key(p.Owner, p.Repo)] = true
		}
	}
	return out
}
