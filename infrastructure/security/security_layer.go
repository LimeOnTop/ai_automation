package security

import (
	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"
	"strings"
)

type securityLayer struct{}

// NewSecurityLayer - creates new security layer
func NewSecurityLayer() interfaces.Security {
	return &securityLayer{}
}

// IsDestructiveAction - checks if action is destructive
func (s *securityLayer) IsDestructiveAction(action entities.AgentResponse) bool {
	destructiveActions := []string{
		"delete", "удалить", "удаление",
		"pay", "оплатить", "оплата", "payment",
		"submit", "отправить", "подтвердить",
		"confirm", "подтверждение",
		"purchase", "купить", "покупка",
		"send", "отправить",
		"remove", "удалить",
		"cancel", "отменить",
	}

	actionLower := strings.ToLower(action.Action)
	reasoningLower := strings.ToLower(action.Reasoning)

	for _, destructive := range destructiveActions {
		if strings.Contains(actionLower, destructive) || strings.Contains(reasoningLower, destructive) {
			return true
		}
	}

	if params, ok := action.Parameters["action"].(string); ok {
		paramsLower := strings.ToLower(params)
		for _, destructive := range destructiveActions {
			if strings.Contains(paramsLower, destructive) {
				return true
			}
		}
	}

	return false
}

// ShouldAskForConfirmation - checks if action requires confirmation
func (s *securityLayer) ShouldAskForConfirmation(action entities.AgentResponse) bool {
	if s.IsDestructiveAction(action) {
		return true
	}

	return false
}
