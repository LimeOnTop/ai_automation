package storage

import (
	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"
	"encoding/json"
	"os"
	"path/filepath"
)

type browserState struct {
	statePath  string
	historyPath string
}

// NewBrowserState - creates new browser state storage
func NewBrowserState() interfaces.Storage {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".ai_automation")
	os.MkdirAll(stateDir, 0755)

	return &browserState{
		statePath:  filepath.Join(stateDir, "state.json"),
		historyPath: filepath.Join(stateDir, "history.json"),
	}
}

// SaveState - saves browser state to file
func (s *browserState) SaveState(state map[string]interface{}) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(s.statePath, data, 0644)
}

// LoadState - loads browser state from file
func (s *browserState) LoadState() (map[string]interface{}, error) {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return state, nil
}

// SaveHistory - saves agent action history
func (s *browserState) SaveHistory(history []entities.AgentResponse) error {
	data, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return os.WriteFile(s.historyPath, data, 0644)
}

// LoadHistory - loads agent action history
func (s *browserState) LoadHistory() ([]entities.AgentResponse, error) {
	data, err := os.ReadFile(s.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []entities.AgentResponse{}, nil
		}
		return nil, err
	}

	var history []entities.AgentResponse
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}
