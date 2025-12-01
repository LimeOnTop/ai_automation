package entities

// AgentResponse представляет ответ агента
type AgentResponse struct {
	Action      string                 `json:"action"`      // Тип действия
	Reasoning   string                 `json:"reasoning"`  // Объяснение решения
	Parameters  map[string]interface{} `json:"parameters"` // Параметры действия
	NeedsInput  bool                   `json:"needs_input"` // Требуется ли ввод от пользователя
	UserMessage string                 `json:"user_message"` // Сообщение для пользователя
	IsComplete  bool                   `json:"is_complete"` // Задача выполнена
}
