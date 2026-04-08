package pinning

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const pinStateFileName = "pins.json"

type Pin struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
}

type PinState struct {
	Pins []Pin `json:"pins"`
}

func PinStatePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".csaw-stash", pinStateFileName)
}

func Read(projectRoot string) (PinState, error) {
	content, err := os.ReadFile(PinStatePath(projectRoot))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PinState{}, nil
		}
		return PinState{}, err
	}

	var state PinState
	if err := json.Unmarshal(content, &state); err != nil {
		return PinState{}, err
	}
	return state, nil
}

func Write(projectRoot string, state PinState) error {
	path := PinStatePath(projectRoot)
	if len(state.Pins) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(path, content, 0o644)
}

func Set(state PinState, source, ref string) PinState {
	for i, pin := range state.Pins {
		if pin.Source == source {
			state.Pins[i].Ref = ref
			return state
		}
	}
	state.Pins = append(state.Pins, Pin{Source: source, Ref: ref})
	return state
}

func Remove(state PinState, source string) PinState {
	filtered := state.Pins[:0]
	for _, pin := range state.Pins {
		if pin.Source != source {
			filtered = append(filtered, pin)
		}
	}
	state.Pins = filtered
	return state
}

func Get(state PinState, source string) (string, bool) {
	for _, pin := range state.Pins {
		if pin.Source == source {
			return pin.Ref, true
		}
	}
	return "", false
}
