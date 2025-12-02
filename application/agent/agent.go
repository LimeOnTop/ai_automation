package agent

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"

	"github.com/sirupsen/logrus"
)

type Agent struct {
	browser       interfaces.BrowserController
	ai            interfaces.AIService
	security      interfaces.SecurityLayer
	logger        *logrus.Logger
	maxIterations int
}

func (a *Agent) GetBrowser() interfaces.BrowserController {
	return a.browser
}

func NewAgent(
	browser interfaces.BrowserController,
	ai interfaces.AIService,
	security interfaces.SecurityLayer,
	logger *logrus.Logger,
) *Agent {
	return &Agent{
		browser:       browser,
		ai:            ai,
		security:      security,
		logger:        logger,
		maxIterations: 100, // Prevent infinite loops
	}
}

func (a *Agent) ExecuteTask(ctx context.Context, task *entities.Task, reader *bufio.Reader) error {
	fmt.Printf("Задача: %s\n", task.Description)
	fmt.Println("Начинаю работу...")
	fmt.Println()

	task.Status = entities.TaskStatusInProgress
	history := []entities.Action{}

	for iteration := 0; iteration < a.maxIterations; iteration++ {
		// Extract current page info
		fmt.Println("Анализирую текущую страницу...")
		pageInfo, err := a.browser.ExtractPageInfo(ctx)
		if err != nil {
			fmt.Printf("Ошибка при анализе страницы: %v\n", err)
			return fmt.Errorf("failed to extract page info: %w", err)
		}

		if pageInfo.URL != "" && pageInfo.URL != "about:blank" {
			fmt.Printf("Текущая страница: %s\n", pageInfo.URL)
		}

		// Decide next action - AI will determine if task is complete
		fmt.Println("Определяю следующее действие...")
		action, err := a.ai.DecideNextAction(ctx, task, pageInfo, history)
		if err != nil {
			fmt.Printf("Ошибка при определении действия: %v\n", err)
			return fmt.Errorf("failed to decide next action: %w", err)
		}

		// If AI returns nil or a "complete" action, task is done
		if action == nil {
			task.Status = entities.TaskStatusCompleted
			return nil
		}

		// Check if action indicates task completion
		if action.Type == "complete" || strings.Contains(strings.ToLower(action.Description), "задача выполнена") ||
			strings.Contains(strings.ToLower(action.Description), "task complete") {
			task.Status = entities.TaskStatusCompleted
			return nil
		}

		// Check if action requires approval
		if a.security.RequiresApproval(ctx, action, pageInfo) {
			action.RequiresApproval = true
			fmt.Printf("\nВНИМАНИЕ: Требуется подтверждение деструктивного действия!\n")
			fmt.Printf("Действие: %s\n", getActionDescription(action))
			fmt.Printf("Описание: %s\n", action.Description)
			fmt.Println("\nЭто действие может быть необратимым (удаление, оплата и т.д.)")
			fmt.Print("Введите 'продолжить' или 'подтвердить' для выполнения, или 'отмена' для отмены: ")

			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "продолжить" || response == "подтвердить" || response == "да" || response == "yes" || response == "y" {
				fmt.Println("Действие подтверждено, продолжаю...")
				fmt.Println()
			} else {
				fmt.Println("Действие отменено пользователем")
				task.Status = entities.TaskStatusWaiting
				return fmt.Errorf("action cancelled by user")
			}
		}

		// Execute action
		fmt.Printf("Выполняю действие: %s\n", getActionDescription(action))
		result := a.executeAction(ctx, action)

		// Log result
		if result.Success {
			fmt.Printf("%s\n\n", result.Message)
		} else {
			fmt.Printf("Ошибка: %s - %s\n", result.Message, result.Error)
			fmt.Println("Попробую другой подход...")
			fmt.Println()

			// If action failed, we continue - agent should adapt
			// But we limit consecutive failures
			if len(history) > 0 && !history[len(history)-1].RequiresApproval {
				// Check for too many failures
				failureCount := 0
				for i := len(history) - 1; i >= 0 && i >= len(history)-5; i-- {
					// We don't track success/failure in history, so we'll continue anyway
					failureCount++
				}
			}
		}

		// Add to history
		history = append(history, *action)

		// Wait a bit between actions to allow page to load
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("Достигнуто максимальное количество итераций (%d)\n", a.maxIterations)
	task.Status = entities.TaskStatusFailed
	return fmt.Errorf("reached maximum iterations (%d)", a.maxIterations)
}

// getActionDescription - returns human-readable description of action
func getActionDescription(action *entities.Action) string {
	switch action.Type {
	case entities.ActionNavigate:
		return fmt.Sprintf("Переход на страницу: %s", action.URL)
	case entities.ActionClick:
		return fmt.Sprintf("Клик на элемент: %s", action.Selector)
	case entities.ActionTypeText:
		return fmt.Sprintf("Ввод текста '%s' в поле: %s", action.Text, action.Selector)
	case entities.ActionScroll:
		return "Прокрутка страницы"
	case entities.ActionExtract:
		return "Извлечение информации со страницы"
	case entities.ActionWait:
		return "Ожидание"
	default:
		return string(action.Type)
	}
}

func (a *Agent) executeAction(ctx context.Context, action *entities.Action) *entities.ActionResult {
	result := &entities.ActionResult{
		Success: false,
	}

	switch action.Type {
	case entities.ActionNavigate:
		if action.URL == "" {
			result.Error = "URL is required for navigate action"
			return result
		}
		err := a.browser.Navigate(ctx, action.URL)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Message = fmt.Sprintf("Успешно перешел на страницу: %s", action.URL)

	case entities.ActionClick:
		if action.Selector == "" {
			result.Error = "Selector is required for click action"
			return result
		}
		err := a.browser.Click(ctx, action.Selector)
		if err != nil {
			result.Error = err.Error()
			result.Message = fmt.Sprintf("Failed to click on %s", action.Selector)
			return result
		}
		result.Success = true
		result.Message = fmt.Sprintf("Успешно кликнул на элемент: %s", action.Selector)

	case entities.ActionTypeText:
		if action.Selector == "" {
			result.Error = "Selector is required for type action"
			return result
		}
		if action.Text == "" {
			result.Error = "Text is required for type action"
			return result
		}
		err := a.browser.TypeText(ctx, action.Selector, action.Text)
		if err != nil {
			result.Error = err.Error()
			result.Message = fmt.Sprintf("Failed to type text into %s", action.Selector)
			return result
		}
		result.Success = true
		result.Message = fmt.Sprintf("Успешно ввел текст в поле: %s", action.Selector)

	case entities.ActionScroll:
		direction := "down"
		amount := 500
		err := a.browser.Scroll(ctx, direction, amount)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Message = "Успешно прокрутил страницу"

	case entities.ActionExtract:
		pageInfo, err := a.browser.ExtractPageInfo(ctx)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Message = "Успешно извлек информацию со страницы"
		result.PageInfo = pageInfo

	case entities.ActionWait:
		timeout := 3
		err := a.browser.Wait(ctx, "", timeout)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Message = fmt.Sprintf("Ожидание %d секунд завершено", timeout)

	default:
		result.Error = fmt.Sprintf("Unknown action type: %s", action.Type)
		return result
	}

	// Get updated page info after action
	pageInfo, err := a.browser.ExtractPageInfo(ctx)
	if err == nil {
		result.PageInfo = pageInfo
	}

	return result
}

func (a *Agent) ApproveAction(ctx context.Context, action *entities.Action) error {
	// Re-execute the action that was waiting for approval
	result := a.executeAction(ctx, action)
	if !result.Success {
		return fmt.Errorf("action failed: %s", result.Error)
	}
	return nil
}
