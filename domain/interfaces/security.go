package interfaces

import "ai_automation/domain/entities"

// Security представляет интерфейс для проверки безопасности действий
type Security interface {
	// IsDestructiveAction проверяет, является ли действие деструктивным
	IsDestructiveAction(action entities.AgentResponse) bool
	
	// ShouldAskForConfirmation определяет, нужно ли спрашивать подтверждение
	ShouldAskForConfirmation(action entities.AgentResponse) bool
}
