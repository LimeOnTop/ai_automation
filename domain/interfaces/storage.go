package interfaces

import "ai_automation/domain/entities"

// Storage представляет интерфейс для хранения состояния
type Storage interface {
	// SaveState сохраняет состояние браузера
	SaveState(state map[string]interface{}) error
	
	// LoadState загружает состояние браузера
	LoadState() (map[string]interface{}, error)
	
	// SaveHistory сохраняет историю действий
	SaveHistory(history []entities.AgentResponse) error
	
	// LoadHistory загружает историю действий
	LoadHistory() ([]entities.AgentResponse, error)
}
