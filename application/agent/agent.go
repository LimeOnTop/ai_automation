package agent

import (
	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"
	"context"
	"fmt"
	"strings"
	"time"
)

type Agent struct {
	ai       interfaces.AI
	browser  interfaces.Browser
	security interfaces.Security
	history  []entities.AgentResponse
	task     string
}

// NewAgent - creates new agent instance
func NewAgent(ai interfaces.AI, browser interfaces.Browser, security interfaces.Security) *Agent {
	return &Agent{
		ai:       ai,
		browser:  browser,
		security: security,
		history:  make([]entities.AgentResponse, 0),
	}
}

// ExecuteTask - executes task autonomously
func (a *Agent) ExecuteTask(ctx context.Context, task string) error {
	a.task = task
	a.history = make([]entities.AgentResponse, 0)

	fmt.Printf("Агент начал выполнение задачи: %s\n\n", task)

	maxIterations := 50
	iteration := 0

	for iteration < maxIterations {

		select {
		case <-ctx.Done():
			return fmt.Errorf("task canceled: %w", ctx.Err())
		default:
		}

		iteration++

		pageInfo, err := a.browser.GetPageInfo(ctx)
		if err != nil {

			if ctx.Err() != nil {
				return fmt.Errorf("task canceled: %w", ctx.Err())
			}
			return fmt.Errorf("failed to get page info: %w", err)
		}

		tabsCount := a.browser.GetTabsCount()
		currentTabIndex := a.browser.GetCurrentTabIndex()
		if tabsCount > 1 {
			pageInfo.Description += fmt.Sprintf("\n\nОткрыто вкладок: %d. Текущая вкладка: %d (0 - первая). Используй switch_to_tab для переключения между вкладками.", tabsCount, currentTabIndex)
		}

		response, err := a.ai.DecideNextAction(ctx, task, pageInfo, a.history)
		if err != nil {
			return fmt.Errorf("failed to decide next action: %w", err)
		}

		a.history = append(a.history, response)

		fmt.Printf("Действие: %s\n", response.Action)
		fmt.Printf("Рассуждение: %s\n", response.Reasoning)
		fmt.Println()

		if response.NeedsInput {
			fmt.Printf("%s\n", response.UserMessage)
			return fmt.Errorf("requires user input")
		}

		if response.IsComplete {
			fmt.Printf("%s\n", response.UserMessage)

			a.browser.SaveState()
			return nil
		}

		if a.security.ShouldAskForConfirmation(response) {
			fmt.Printf("Требуется подтверждение для действия: %s\n", response.Action)
			fmt.Printf("Причина: %s\n", response.Reasoning)
			return fmt.Errorf("requires confirmation")
		}

		if err := a.executeAction(ctx, response); err != nil {

			fmt.Printf("Ошибка при выполнении действия: %v\n", err)

			if response.Action == "wait_for_element" {

				if pageInfo, err := a.browser.GetPageInfo(ctx); err == nil {

					var availableSelectors []string
					for _, el := range pageInfo.Elements {
						if el.IsVisible && el.IsClickable {
							availableSelectors = append(availableSelectors, el.Selector)
							if len(availableSelectors) >= 10 {
								break
							}
						}
					}
					errorResponse := entities.AgentResponse{
						Action:    "error",
						Reasoning: fmt.Sprintf("Элемент не найден: %v. Доступные элементы на странице: %s. Попробую использовать другой селектор или подход.", err, strings.Join(availableSelectors, ", ")),
					}
					a.history = append(a.history, errorResponse)
				} else {
					errorResponse := entities.AgentResponse{
						Action:    "error",
						Reasoning: fmt.Sprintf("Ошибка: %v. Попробую другой подход.", err),
					}
					a.history = append(a.history, errorResponse)
				}
			} else {

				errorResponse := entities.AgentResponse{
					Action:    "error",
					Reasoning: fmt.Sprintf("Ошибка: %v. Попробую другой подход.", err),
				}
				a.history = append(a.history, errorResponse)
			}

			select {
			case <-ctx.Done():

				a.browser.SaveState()
				return fmt.Errorf("task canceled: %w", ctx.Err())
			case <-time.After(1 * time.Second):
			}
			continue
		}

		select {
		case <-ctx.Done():
			a.browser.SaveState()
			return fmt.Errorf("task canceled: %w", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}

		if err := a.browser.SaveState(); err != nil {

			fmt.Printf("Предупреждение: не удалось сохранить состояние браузера: %v\n", err)
		}
	}

	return fmt.Errorf("max iterations reached")
}

// ExecuteActionWithConfirmation выполняет действие после подтверждения
// ExecuteActionWithConfirmation - executes action after confirmation
func (a *Agent) ExecuteActionWithConfirmation(ctx context.Context, response entities.AgentResponse) error {
	if action, ok := response.Parameters["action"].(string); ok {
		response.Action = action
		if params, ok := response.Parameters["parameters"].(map[string]interface{}); ok {
			response.Parameters = params
		}
	}

	return a.executeAction(ctx, response)
}

// executeAction - executes single action
func (a *Agent) executeAction(ctx context.Context, response entities.AgentResponse) error {
	switch response.Action {
	case "navigate":
		url, ok := response.Parameters["url"].(string)
		if !ok {
			return fmt.Errorf("url parameter is required for navigate action")
		}
		fmt.Printf("Переход на: %s\n", url)
		return a.browser.Navigate(ctx, url)

	case "open_new_tab":
		url, _ := response.Parameters["url"].(string)
		if url != "" {
			fmt.Printf("Открытие новой вкладки и переход на: %s\n", url)
		} else {
			fmt.Printf("Открытие новой вкладки\n")
		}
		return a.browser.OpenNewTab(ctx, url)

	case "switch_to_tab":
		index, ok := response.Parameters["index"]
		if !ok {
			return fmt.Errorf("index parameter is required for switch_to_tab action")
		}
		var tabIndex int
		switch v := index.(type) {
		case int:
			tabIndex = v
		case float64:
			tabIndex = int(v)
		default:
			return fmt.Errorf("index must be an integer")
		}
		fmt.Printf("Переключение на вкладку %d\n", tabIndex)
		return a.browser.SwitchToTab(tabIndex)

	case "click":
		selector, ok := response.Parameters["selector"].(string)
		if !ok {
			return fmt.Errorf("selector parameter is required for click action")
		}
		fmt.Printf("Клик на элемент: %s\n", selector)
		return a.browser.Click(ctx, selector)

	case "type":
		selector, ok := response.Parameters["selector"].(string)
		if !ok {
			return fmt.Errorf("selector parameter is required for type action")
		}
		text, ok := response.Parameters["text"].(string)
		if !ok {
			return fmt.Errorf("text parameter is required for type action")
		}
		fmt.Printf("Ввод текста в %s: %s\n", selector, text)
		return a.browser.Type(ctx, selector, text)

	case "wait_for_element":
		selector, ok := response.Parameters["selector"].(string)
		if !ok {
			return fmt.Errorf("selector parameter is required for wait_for_element action")
		}
		fmt.Printf("Ожидание элемента: %s\n", selector)
		return a.browser.WaitForElement(ctx, selector)

	case "complete":

		return nil

	default:
		return fmt.Errorf("unknown action: %s", response.Action)
	}
}

// GetHistory возвращает историю действий
// GetHistory - returns action history
func (a *Agent) GetHistory() []entities.AgentResponse {
	return a.history
}

// GetPendingAction возвращает последнее действие, требующее подтверждения
// GetPendingAction - returns pending action requiring confirmation
func (a *Agent) GetPendingAction() *entities.PendingAction {
	for i := len(a.history) - 1; i >= 0; i-- {
		if a.history[i].NeedsInput || a.security.ShouldAskForConfirmation(a.history[i]) {
			return &entities.PendingAction{
				Action:     a.history[i].Action,
				Reasoning:  a.history[i].Reasoning,
				Parameters: a.history[i].Parameters,
			}
		}
	}
	return nil
}

// GetTask возвращает текущую задачу
// GetTask - returns current task
func (a *Agent) GetTask() string {
	return a.task
}
