//go:build !windows

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// script writes an executable shell script and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prog")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) emit(event string, _ any) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *recorder) count(event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e == event {
			n++
		}
	}
	return n
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met within 5s")
}

func TestLaunchStreamsBothStreamsAndExitCode(t *testing.T) {
	rec := &recorder{}
	m := NewManager(rec.emit)

	bin := script(t, "echo hello-out\necho hello-err 1>&2\nexit 3\n")
	proc, err := m.Launch(Spec{Owner: "playdead", Repo: "dumpscope", BinaryPath: bin})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if !proc.Running || proc.PID == 0 {
		t.Fatalf("launched process looks wrong: %+v", proc)
	}

	waitFor(t, func() bool {
		list := m.List()
		return len(list) == 1 && !list[0].Running
	})

	final := m.List()[0]
	if final.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3", final.ExitCode)
	}
	if final.ExitedAt.IsZero() {
		t.Error("ExitedAt not recorded")
	}

	lines, err := m.Logs(proc.ID)
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut, sys int
	for _, l := range lines {
		switch l.Stream {
		case StreamStdout:
			out++
			if l.Text != "hello-out" {
				t.Errorf("stdout text = %q", l.Text)
			}
		case StreamStderr:
			errOut++
			if l.Text != "hello-err" {
				t.Errorf("stderr text = %q", l.Text)
			}
		case StreamSystem:
			sys++
		}
	}
	if out != 1 || errOut != 1 {
		t.Errorf("stdout=%d stderr=%d, want 1 each", out, errOut)
	}
	if sys < 2 {
		t.Errorf("expected start and exit narration, got %d system lines", sys)
	}
	if rec.count(EventLog) < 4 {
		t.Errorf("log events = %d", rec.count(EventLog))
	}
	if rec.count(EventProcess) < 2 {
		t.Errorf("process events = %d, want start and exit", rec.count(EventProcess))
	}

	// Sequence numbers must be strictly increasing so the UI can order lines.
	for i := 1; i < len(lines); i++ {
		if lines[i].Seq <= lines[i-1].Seq {
			t.Fatalf("sequence not increasing at %d: %d then %d", i, lines[i-1].Seq, lines[i].Seq)
		}
	}
}

func TestStopTerminatesRunningProcess(t *testing.T) {
	m := NewManager(nil)
	bin := script(t, "trap 'exit 0' TERM\nwhile true; do sleep 0.05; done\n")

	proc, err := m.Launch(Spec{Repo: "sleeper", BinaryPath: bin})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if m.RunningCount() != 1 {
		t.Fatalf("running count = %d", m.RunningCount())
	}

	if err := m.Stop(proc.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitFor(t, func() bool { return m.RunningCount() == 0 })

	// Stopping an already-stopped process is a no-op, not an error.
	if err := m.Stop(proc.ID); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestStopKillsProcessThatIgnoresTerm(t *testing.T) {
	m := NewManager(nil)
	m.SetGrace(300*time.Millisecond, time.Second)
	bin := script(t, "trap '' TERM\necho ready\nwhile true; do sleep 0.05; done\n")

	proc, err := m.Launch(Spec{Repo: "stubborn", BinaryPath: bin})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	// Wait until the trap is installed, otherwise the signal races startup and
	// the default disposition kills the shell before it can ignore it.
	waitFor(t, func() bool {
		lines, _ := m.Logs(proc.ID)
		return containsText(lines, "ready")
	})

	start := time.Now()
	if err := m.Stop(proc.ID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Fatalf("Stop returned after %v — expected it to wait out the grace period", elapsed)
	}
	waitFor(t, func() bool { return m.RunningCount() == 0 })

	lines, _ := m.Logs(proc.ID)
	if !hasSystemText(lines, "killing") {
		t.Errorf("expected the escalation to be narrated: %v", texts(lines))
	}
}

func hasSystemText(lines []Line, want string) bool {
	for _, l := range lines {
		if l.Stream == StreamSystem && strings.Contains(l.Text, want) {
			return true
		}
	}
	return false
}

func TestMultipleConcurrentProcesses(t *testing.T) {
	m := NewManager(nil)
	bin := script(t, "echo up\nsleep 30\n")

	a, err := m.Launch(Spec{Repo: "one", BinaryPath: bin})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.Launch(Spec{Repo: "two", BinaryPath: bin})
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("ids must be unique")
	}
	waitFor(t, func() bool { return m.RunningCount() == 2 })

	if got := len(m.List()); got != 2 {
		t.Fatalf("List returned %d", got)
	}

	t.Cleanup(func() { m.StopAll(context.Background()) })
}

func TestLaunchTwiceGivesSeparateLogs(t *testing.T) {
	m := NewManager(nil)
	first, err := m.Launch(Spec{Repo: "app", BinaryPath: script(t, "echo first\n")})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.Launch(Spec{Repo: "app", BinaryPath: script(t, "echo second\n")})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return m.RunningCount() == 0 })

	firstLines, _ := m.Logs(first.ID)
	secondLines, _ := m.Logs(second.ID)
	if !containsText(firstLines, "first") || containsText(firstLines, "second") {
		t.Errorf("first log leaked: %v", texts(firstLines))
	}
	if !containsText(secondLines, "second") {
		t.Errorf("second log missing its line: %v", texts(secondLines))
	}
}

func TestClearAndForget(t *testing.T) {
	m := NewManager(nil)
	proc, err := m.Launch(Spec{Repo: "app", BinaryPath: script(t, "echo noise\n")})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return m.RunningCount() == 0 })

	if err := m.Clear(proc.ID); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	lines, _ := m.Logs(proc.ID)
	if len(lines) != 0 {
		t.Fatalf("Clear left %d lines", len(lines))
	}

	if err := m.Forget(proc.ID); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatal("Forget did not remove the process")
	}
	if _, err := m.Logs(proc.ID); err == nil {
		t.Fatal("expected ErrNotFound after Forget")
	}
}

func TestLaunchMissingBinary(t *testing.T) {
	m := NewManager(nil)
	if _, err := m.Launch(Spec{Repo: "ghost", BinaryPath: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("expected an error launching a missing binary")
	}
	if _, err := m.Launch(Spec{Repo: "ghost"}); err == nil {
		t.Fatal("expected an error for an empty binary path")
	}
}

func TestLogsAreCapped(t *testing.T) {
	m := NewManager(nil)
	// Emit more lines than the retention cap.
	bin := script(t, "i=0\nwhile [ $i -lt 5200 ]; do echo line-$i; i=$((i+1)); done\n")
	proc, err := m.Launch(Spec{Repo: "chatty", BinaryPath: bin})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return m.RunningCount() == 0 })

	lines, _ := m.Logs(proc.ID)
	if len(lines) > maxLines {
		t.Fatalf("retained %d lines, cap is %d", len(lines), maxLines)
	}
	if len(lines) < maxLines {
		t.Fatalf("expected the log to fill to the cap, got %d", len(lines))
	}
	// The tail must be kept, not the head.
	if !strings.Contains(lines[len(lines)-1].Text, "exited") && !strings.Contains(lines[len(lines)-1].Text, "line-5199") {
		t.Fatalf("last line = %q, expected recent output", lines[len(lines)-1].Text)
	}
}

func TestUptime(t *testing.T) {
	p := Process{StartedAt: time.Now().Add(-2 * time.Second), Running: true}
	if p.Uptime() < 2*time.Second {
		t.Fatalf("uptime = %v", p.Uptime())
	}
	p = Process{StartedAt: time.Now().Add(-10 * time.Second), ExitedAt: time.Now().Add(-4 * time.Second)}
	if got := p.Uptime().Round(time.Second); got != 6*time.Second {
		t.Fatalf("uptime = %v, want 6s", got)
	}
}

func containsText(lines []Line, want string) bool {
	for _, l := range lines {
		if l.Stream != StreamSystem && strings.Contains(l.Text, want) {
			return true
		}
	}
	return false
}

func texts(lines []Line) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return out
}
