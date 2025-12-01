package entities

// PendingAction представляет действие, требующее подтверждения
type PendingAction struct {
	Action     string                 `json:"action"`
	Reasoning  string                 `json:"reasoning"`
	Parameters map[string]interface{} `json:"parameters"`
}
