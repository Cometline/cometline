package processctl

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	Mode         string `json:"mode"`
	PID          int    `json:"pid"`
	StartedAt    string `json:"started_at"`
	DataDir      string `json:"data_dir"`
	SettingsPath string `json:"settings_path"`
}

type State struct {
	Metadata Metadata `json:"metadata"`
	Present  bool     `json:"present"`
	Running  bool     `json:"running"`
	Stale    bool     `json:"stale"`
}

func KnownModes() []string {
	return []string{ModeServe, ModeGatewayDiscord}
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
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(metaPath, data, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(meta.PID)+"\n"), 0o600); err != nil {
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
