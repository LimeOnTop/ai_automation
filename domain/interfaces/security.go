package interfaces

import (
	"ai_automation/domain/entities"
	"context"
)

// SecurityLayer defines the interface for security checks
type SecurityLayer interface {
	// RequiresApproval checks if an action requires user approval
	RequiresApproval(ctx context.Context, action *entities.Action, pageInfo *entities.PageInfo) bool
	
	// IsDestructiveAction checks if an action is destructive
	IsDestructiveAction(ctx context.Context, action *entities.Action) bool
	
	// GetActionRiskLevel returns the risk level of an action
	GetActionRiskLevel(ctx context.Context, action *entities.Action) string
}

