package interfaces

import (
	"ai_automation/domain/entities"
	"context"
)

// AI представляет интерфейс для работы с AI моделью
type AI interface {
	// DecideNextAction принимает решение о следующем действии на основе контекста
	DecideNextAction(ctx context.Context, task string, pageInfo entities.PageInfo, history []entities.AgentResponse) (entities.AgentResponse, error)
	
	// AnalyzePage анализирует страницу и извлекает ключевую информацию
	AnalyzePage(ctx context.Context, pageInfo entities.PageInfo, task string) (string, error)
}
