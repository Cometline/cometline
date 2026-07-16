package processctl

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/paths"
)

const (
	ModeServe          = "serve"
	ModeGatewayDiscord = "gateway-discord"
)

type Metadata struct {
	Mode         string   `json:"mode"`
	PID          int      `json:"pid"`
	StartedAt    string   `json:"started_at"`
	DataDir      string   `json:"data_dir"`
	SettingsPath string   `json:"settings_path"`
	CmdArgs      []string `json:"cmd_args,omitempty"`
}

type State struct {
	Metadata Metadata `json:"metadata"`
	Present  bool     `json:"present"`
	Running  bool     `json:"running"`
	Stale    bool     `json:"stale"`
}

// ReloadResult is written by a long-lived process (serve/gateway) after it
// finishes handling a SIGHUP-triggered settings reload. A short-lived CLI
// invocation (`cometmind settings reload`) polls this file so it can report
// whether the reload actually succeeded instead of just "signal delivered".
type ReloadResult struct {
	// Generation increments on every reload attempt, success or failure, so a
	// poller can detect "a new reload happened" even if two attempts produce
	// the same Success value.
	Generation int64  `json:"generation"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	FinishedAt string `json:"finished_at"`
}

func KnownModes() []string {
	return []string{ModeServe, ModeGatewayDiscord}
}

// GatewayModes returns process modes for gateway platforms (not serve).
// Add new platforms here when additional gateways ship.
func GatewayModes() []string {
	return []string{ModeGatewayDiscord}
}

func TargetModes(args []string) ([]string, error) {
	if len(args) == 0 {
		return KnownModes(), nil
	}
	out := make([]string, 0, len(args))
	seen := make(map[string]struct{}, len(args))
	for _, mode := range args {
		if !isKnownMode(mode) {
			return nil, fmt.Errorf("unknown process mode %q", mode)
		}
		if _, ok := seen[mode]; ok {
			continue
		}
		seen[mode] = struct{}{}
		out = append(out, mode)
	}
	return out, nil
}

func WriteMetadata(mode string) error {
	if !isKnownMode(mode) {
		return fmt.Errorf("unknown process mode %q", mode)
	}
	metaPath, err := paths.ProcessMetaPath(mode)
	if err != nil {
		return err
	}
	pidPath, err := paths.ProcessPIDPath(mode)
	if err != nil {
		return err
	}
	dataDir, err := paths.DataDir()
	if err != nil {
		return err
	}
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return err
	}
	meta := Metadata{
		Mode:         mode,
		PID:          os.Getpid(),
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
		DataDir:      dataDir,
		SettingsPath: settingsPath,
		CmdArgs:      os.Args,
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := writeFileAtomic(metaPath, data, 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(pidPath, []byte(strconv.Itoa(meta.PID)+"\n"), 0o600); err != nil {
		return err
	}
	return nil
}

func RemoveMetadata(mode string) {
	if metaPath, err := paths.ProcessMetaPath(mode); err == nil {
		_ = os.Remove(metaPath)
	}
	if pidPath, err := paths.ProcessPIDPath(mode); err == nil {
		_ = os.Remove(pidPath)
	}
}

// ReadReloadResult reads the last-recorded reload outcome for one process
// mode. A missing file (no reload has happened yet this process lifetime)
// returns a zero-value result with Generation 0, not an error.
func ReadReloadResult(mode string) (ReloadResult, error) {
	if !isKnownMode(mode) {
		return ReloadResult{}, fmt.Errorf("unknown process mode %q", mode)
	}
	path, err := reloadResultPath(mode)
	if err != nil {
		return ReloadResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ReloadResult{}, nil
		}
		return ReloadResult{}, err
	}
	var result ReloadResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ReloadResult{}, err
	}
	return result, nil
}

// WriteReloadResult persists the outcome of one reload attempt. It is
// intentionally best-effort from the caller's perspective (the runtime reload
// itself already succeeded or failed; this just records that fact) but
// returns the write error so callers can log it.
func WriteReloadResult(mode string, result ReloadResult) error {
	if !isKnownMode(mode) {
		return fmt.Errorf("unknown process mode %q", mode)
	}
	path, err := reloadResultPath(mode)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0o600)
}

// WaitForReloadGeneration polls the reload-result file for one process mode
// until its Generation exceeds sinceGeneration (a new reload attempt has been
// recorded) or timeout elapses. It returns the observed result and whether a
// new generation was seen before the timeout.
func WaitForReloadGeneration(mode string, sinceGeneration int64, timeout time.Duration) (ReloadResult, bool) {
	deadline := time.Now().Add(timeout)
	for {
		result, err := ReadReloadResult(mode)
		if err == nil && result.Generation > sinceGeneration {
			return result, true
		}
		if time.Now().After(deadline) {
			return result, false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func reloadResultPath(mode string) (string, error) {
	return paths.ProcessReloadResultPath(mode)
}

// writeFileAtomic writes data to path via a temp file + rename so a reader
// (e.g. a concurrent poller) never observes a partially-written file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Signal(sig syscall.Signal, modes ...string) (int, error) {
	count := 0
	for _, mode := range modes {
		state, err := ReadState(mode)
		if err != nil {
			return count, err
		}
		if !state.Running {
			continue
		}
		proc, err := os.FindProcess(state.Metadata.PID)
		if err != nil {
			return count, err
		}
		if err := proc.Signal(sig); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				continue
			}
			return count, err
		}
		count++
	}
	return count, nil
}

// Restart stops the target process and relaunches it using its recorded CmdArgs.
// It waits up to timeout for the process to exit before relaunching.
func Restart(mode string, timeout time.Duration) error {
	state, err := ReadState(mode)
	if err != nil {
		return err
	}
	if state.Running {
		proc, err := os.FindProcess(state.Metadata.PID)
		if err != nil {
			return fmt.Errorf("find process: %w", err)
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("signal process: %w", err)
		}
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			s, _ := ReadState(mode)
			if !s.Running {
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
		// Force-kill if still alive after timeout.
		if s, _ := ReadState(mode); s.Running {
			_ = proc.Signal(syscall.SIGKILL)
			time.Sleep(300 * time.Millisecond)
		}
	}

	args := state.Metadata.CmdArgs
	if len(args) == 0 {
		return fmt.Errorf("no cmd_args recorded for mode %q; cannot relaunch", mode)
	}
	binary := args[0]
	cmd := exec.Command(binary, args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Start()
}

func ReadState(mode string) (State, error) {
	if !isKnownMode(mode) {
		return State{}, fmt.Errorf("unknown process mode %q", mode)
	}
	metaPath, err := paths.ProcessMetaPath(mode)
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return State{}, err
	}
	state := State{Metadata: meta, Present: true}
	if meta.PID <= 0 {
		state.Stale = true
		return state, nil
	}
	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		state.Stale = true
		return state, nil
	}
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		state.Running = true
		return state, nil
	} else if !strings.Contains(err.Error(), "operation not permitted") {
		state.Stale = true
		return state, nil
	}
	state.Running = true
	return state, nil
}

func isKnownMode(mode string) bool {
	for _, known := range KnownModes() {
		if mode == known {
			return true
		}
	}
	return false
}
