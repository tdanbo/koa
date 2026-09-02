// Package runner launches installed binaries as child processes and streams
// their output into koa's in-app log panel rather than an OS terminal (PRD §11).
package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// maxLines caps a process's retained log. Older lines are dropped so a chatty
// long-running app cannot grow koa's memory without bound.
const maxLines = 5000

// Stream identifies where a log line came from.
type Stream string

const (
	StreamStdout Stream = "stdout"
	StreamStderr Stream = "stderr"
	// StreamSystem is koa's own narration — start, exit, stop.
	StreamSystem Stream = "system"
)

// Line is one entry in a process log panel.
type Line struct {
	Seq    int64     `json:"seq"`
	Time   time.Time `json:"time"`
	Stream Stream    `json:"stream"`
	Text   string    `json:"text"`
}

// Process is the UI-facing view of a launched binary.
type Process struct {
	ID        string    `json:"id"`
	Owner     string    `json:"owner"`
	Repo      string    `json:"repo"`
	Command   string    `json:"command"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"startedAt"`
	ExitedAt  time.Time `json:"exitedAt"`
	Running   bool      `json:"running"`
	ExitCode  int       `json:"exitCode"`
	// Failure is set when the process could not be started or ended badly.
	Failure string `json:"failure"`
}

// Uptime is how long the process has been (or was) alive.
func (p Process) Uptime() time.Duration {
	end := p.ExitedAt
	if p.Running || end.IsZero() {
		end = time.Now()
	}
	return end.Sub(p.StartedAt)
}

// Spec describes one launch.
type Spec struct {
	Owner      string
	Repo       string
	BinaryPath string
	Args       []string
	Dir        string
	Env        []string
}

// Emitter delivers runner events to the frontend. Implementations must be safe
// to call from any goroutine.
type Emitter func(event string, data any)

// Event names emitted by the manager.
const (
	EventLog     = "koa:log"
	EventProcess = "koa:process"
)

type proc struct {
	mu sync.Mutex

	info  Process
	cmd   *exec.Cmd
	lines []Line
	done  chan struct{}
}

// Manager owns every live child process.
type Manager struct {
	emit Emitter

	// termGrace is how long Stop waits after a polite terminate before
	// escalating to a hard kill; killGrace bounds the wait after that.
	termGrace time.Duration
	killGrace time.Duration

	mu    sync.RWMutex
	order []string
	procs map[string]*proc

	seq atomic.Int64
	ids atomic.Int64
}

// NewManager returns a Manager that reports activity through emit. A nil emit
// is allowed, which is what tests and headless use rely on.
func NewManager(emit Emitter) *Manager {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &Manager{
		emit:      emit,
		procs:     map[string]*proc{},
		termGrace: 5 * time.Second,
		killGrace: 3 * time.Second,
	}
}

// ErrNotFound means no process with that id is tracked.
var ErrNotFound = errors.New("no such process")

// Launch starts the binary described by spec and begins streaming its output.
func (m *Manager) Launch(spec Spec) (Process, error) {
	if spec.BinaryPath == "" {
		return Process{}, errors.New("no binary path to launch")
	}

	id := fmt.Sprintf("%s-%d", spec.Repo, m.ids.Add(1))

	cmd := exec.Command(spec.BinaryPath, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	configureProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Process{}, fmt.Errorf("capture stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Process{}, fmt.Errorf("capture stderr: %w", err)
	}

	p := &proc{
		info: Process{
			ID:        id,
			Owner:     spec.Owner,
			Repo:      spec.Repo,
			Command:   spec.BinaryPath,
			StartedAt: time.Now(),
			Running:   true,
		},
		cmd:  cmd,
		done: make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return Process{}, fmt.Errorf("launch %s: %w", spec.Repo, err)
	}
	p.info.PID = cmd.Process.Pid

	m.mu.Lock()
	m.procs[id] = p
	m.order = append(m.order, id)
	m.mu.Unlock()

	m.append(p, StreamSystem, fmt.Sprintf("started %s (pid %d)", spec.BinaryPath, p.info.PID))
	m.emit(EventProcess, p.snapshot())

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); m.pump(p, StreamStdout, stdout) }()
	go func() { defer wg.Done(); m.pump(p, StreamStderr, stderr) }()

	go func() {
		wg.Wait()
		err := cmd.Wait()

		p.mu.Lock()
		p.info.Running = false
		p.info.ExitedAt = time.Now()
		var exitErr *exec.ExitError
		switch {
		case err == nil:
			p.info.ExitCode = 0
		case errors.As(err, &exitErr):
			p.info.ExitCode = exitErr.ExitCode()
		default:
			p.info.ExitCode = -1
			p.info.Failure = err.Error()
		}
		code := p.info.ExitCode
		failure := p.info.Failure
		p.mu.Unlock()

		if failure != "" {
			m.append(p, StreamSystem, "process ended: "+failure)
		} else {
			m.append(p, StreamSystem, fmt.Sprintf("process exited with code %d", code))
		}
		m.emit(EventProcess, p.snapshot())
		close(p.done)
	}()

	return p.snapshot(), nil
}

// pump reads a pipe line by line into the process log.
func (m *Manager) pump(p *proc, stream Stream, r io.Reader) {
	scanner := bufio.NewScanner(r)
	// Tolerate long lines (progress bars, JSON blobs) without truncating hard.
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		m.append(p, stream, scanner.Text())
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		m.append(p, StreamSystem, fmt.Sprintf("output stream ended: %v", err))
	}
}

// append records a line and pushes it to the frontend.
func (m *Manager) append(p *proc, stream Stream, text string) {
	line := Line{Seq: m.seq.Add(1), Time: time.Now(), Stream: stream, Text: text}

	p.mu.Lock()
	p.lines = append(p.lines, line)
	if over := len(p.lines) - maxLines; over > 0 {
		p.lines = append(p.lines[:0], p.lines[over:]...)
	}
	id := p.info.ID
	p.mu.Unlock()

	m.emit(EventLog, struct {
		ID   string `json:"id"`
		Line Line   `json:"line"`
	}{ID: id, Line: line})
}

func (p *proc) snapshot() Process {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.info
}

// List returns every tracked process in launch order.
func (m *Manager) List() []Process {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Process, 0, len(m.order))
	for _, id := range m.order {
		if p, ok := m.procs[id]; ok {
			out = append(out, p.snapshot())
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

// RunningCount is how many processes are currently alive, which drives the
// pulsing dot on the Running nav item (PRD §5.2).
func (m *Manager) RunningCount() int {
	n := 0
	for _, p := range m.List() {
		if p.Running {
			n++
		}
	}
	return n
}

// Logs returns a copy of a process's retained log.
func (m *Manager) Logs(id string) ([]Line, error) {
	p, err := m.lookup(id)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Line(nil), p.lines...), nil
}

// Clear empties a process's log panel without touching the process itself.
func (m *Manager) Clear(id string) error {
	p, err := m.lookup(id)
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.lines = nil
	p.mu.Unlock()
	return nil
}

// Stop terminates a running process, escalating to a hard kill if it does not
// exit within grace.
func (m *Manager) Stop(id string) error {
	p, err := m.lookup(id)
	if err != nil {
		return err
	}

	p.mu.Lock()
	running, cmd := p.info.Running, p.cmd
	p.mu.Unlock()
	if !running || cmd == nil || cmd.Process == nil {
		return nil
	}

	m.append(p, StreamSystem, "stopping…")
	if err := terminate(cmd); err != nil {
		return fmt.Errorf("stop %s: %w", id, err)
	}

	select {
	case <-p.done:
		return nil
	case <-time.After(m.termGrace):
		m.append(p, StreamSystem, "process did not exit — killing")
		if err := kill(cmd); err != nil {
			return fmt.Errorf("kill %s: %w", id, err)
		}
		select {
		case <-p.done:
		case <-time.After(m.killGrace):
		}
		return nil
	}
}

// Forget drops a stopped process from the list, closing its log tab. A running
// process is stopped first.
func (m *Manager) Forget(id string) error {
	p, err := m.lookup(id)
	if err != nil {
		return err
	}
	if p.snapshot().Running {
		if err := m.Stop(id); err != nil {
			return err
		}
	}
	m.mu.Lock()
	delete(m.procs, id)
	for i, existing := range m.order {
		if existing == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
	return nil
}

// StopAll terminates every live process. It is called when koa quits so no
// orphaned children are left behind.
func (m *Manager) StopAll(ctx context.Context) {
	for _, info := range m.List() {
		if !info.Running {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		_ = m.Stop(info.ID)
	}
}

func (m *Manager) lookup(id string) (*proc, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.procs[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return p, nil
}

// SetGrace overrides how long Stop waits for a polite exit before killing.
// It exists so tests need not sit through the production grace period.
func (m *Manager) SetGrace(term, hard time.Duration) {
	m.termGrace, m.killGrace = term, hard
}
