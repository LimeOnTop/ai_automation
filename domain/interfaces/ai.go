package interfaces

import (
	"ai_automation/domain/entities"
	"context"
)

// AIService defines the interface for AI decision making
type AIService interface {
	// DecideNextAction decides what action to take next based on task and context
	// Returns nil if task is complete or cannot proceed
	DecideNextAction(ctx context.Context, task *entities.Task, pageInfo *entities.PageInfo, history []entities.Action) (*entities.Action, error)
	
	// AnalyzePage analyzes the page and extracts relevant information
	AnalyzePage(ctx context.Context, pageInfo *entities.PageInfo, task *entities.Task) (string, error)
}

