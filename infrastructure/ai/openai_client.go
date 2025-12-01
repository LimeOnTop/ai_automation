package ai

import (
	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	maxTokens   = 4000
	temperature = 0.7
	model       = "gpt-4o"
	apiURL      = "https://api.openai.com/v1/chat/completions"
)

type openAIClient struct {
	apiKey string
	client *http.Client
}

// NewOpenAIClient - creates new OpenAI client
func NewOpenAIClient() (interfaces.AI, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	return &openAIClient{
		apiKey: apiKey,
		client: &http.Client{},
	}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type functionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// UnmarshalJSON - custom deserializer for functionCall
func (fc *functionCall) UnmarshalJSON(data []byte) error {
	type Alias functionCall
	aux := &struct {
		Arguments interface{} `json:"arguments"`
		*Alias
	}{
		Alias: (*Alias)(fc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if argsStr, ok := aux.Arguments.(string); ok {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
			return fmt.Errorf("failed to parse arguments string: %w", err)
		}
		fc.Arguments = args
	} else if argsMap, ok := aux.Arguments.(map[string]interface{}); ok {

		fc.Arguments = argsMap
	} else {
		fc.Arguments = make(map[string]interface{})
	}

	return nil
}

type chatRequest struct {
	Model        string        `json:"model"`
	Messages     []chatMessage `json:"messages"`
	Functions    []functionDef `json:"functions,omitempty"`
	FunctionCall interface{}   `json:"function_call,omitempty"`
	Temperature  float64       `json:"temperature"`
	MaxTokens    int           `json:"max_tokens"`
}

type functionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Role         string        `json:"role"`
			Content      string        `json:"content"`
			FunctionCall *functionCall `json:"function_call,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

// DecideNextAction - decides next action for the agent
func (c *openAIClient) DecideNextAction(ctx context.Context, task string, pageInfo entities.PageInfo, history []entities.AgentResponse) (entities.AgentResponse, error) {
	systemPrompt := c.buildSystemPrompt()

	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: c.buildUserPrompt(task, pageInfo, history)},
	}

	functions := c.getFunctionDefinitions()

	req := chatRequest{
		Model:        model,
		Messages:     messages,
		Functions:    functions,
		FunctionCall: "auto",
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	}

	resp, err := c.makeRequest(ctx, req)
	if err != nil {
		return entities.AgentResponse{}, err
	}

	if len(resp.Choices) == 0 {
		return entities.AgentResponse{}, fmt.Errorf("no response from OpenAI")
	}

	message := resp.Choices[0].Message

	if message.FunctionCall != nil {
		return c.parseFunctionCall(message.FunctionCall)
	}

	return entities.AgentResponse{
		Action:      "message",
		Reasoning:   message.Content,
		NeedsInput:  true,
		UserMessage: message.Content,
		IsComplete:  c.isTaskComplete(message.Content),
	}, nil
}

// AnalyzePage - analyzes the current page
func (c *openAIClient) AnalyzePage(ctx context.Context, pageInfo entities.PageInfo, task string) (string, error) {
	prompt := fmt.Sprintf(`Проанализируй следующую веб-страницу в контексте задачи: "%s"

URL: %s
Title: %s
Description: %s

Доступные элементы:
%s

Текстовое содержимое (первые 2000 символов):
%s

Извлеки ключевую информацию, которая поможет выполнить задачу. Опиши структуру страницы и доступные действия.`,
		task,
		pageInfo.URL,
		pageInfo.Title,
		pageInfo.Description,
		c.formatElements(pageInfo.Elements),
		pageInfo.TextContent,
	)

	messages := []chatMessage{
		{Role: "system", Content: "Ты эксперт по анализу веб-страниц. Извлекай только релевантную информацию."},
		{Role: "user", Content: prompt},
	}

	req := chatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   1000,
	}

	resp, err := c.makeRequest(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// buildSystemPrompt - builds system prompt for AI
func (c *openAIClient) buildSystemPrompt() string {
	return `Ты автономный AI-агент, который управляет веб-браузером для выполнения задач пользователя.

Твои возможности:
- navigate(url) - перейти на URL в текущей вкладке
- open_new_tab(url) - открыть новую вкладку и перейти на URL (если url пустой, просто открыть новую вкладку)
- switch_to_tab(index) - переключиться на вкладку по индексу (0 - первая вкладка)
- click(selector) - кликнуть на элемент по CSS селектору
- type(selector, text) - ввести текст в поле
- wait_for_element(selector) - подождать появления элемента
- get_page_info() - получить информацию о текущей странице
- analyze_page() - проанализировать страницу

ВАЖНЫЕ ПРАВИЛА:
1. Ты должен САМ определять селекторы элементов, анализируя структуру страницы
2. НЕ используй заготовки действий - каждый раз анализируй страницу заново
3. Если действие может быть деструктивным (удаление, оплата, отправка), используй action="ask_confirmation"
4. Работай автономно, но если нужна дополнительная информация - используй needs_input=true
5. После каждого действия анализируй результат и планируй следующие шаги
6. Если задача выполнена, установи is_complete=true
7. Если ссылка открывается в новой вкладке (target="_blank"), используй open_new_tab или автоматически переключись на новую вкладку
8. При работе с несколькими вкладками, используй switch_to_tab для переключения между ними

Формат ответа: используй function calling для вызова действий.`
}

// buildUserPrompt - builds user prompt with task and page info
func (c *openAIClient) buildUserPrompt(task string, pageInfo entities.PageInfo, history []entities.AgentResponse) string {
	var historyText string
	if len(history) > 0 {
		historyText = "\n\nИстория действий:\n"
		for i, h := range history {
			historyText += fmt.Sprintf("%d. %s: %s\n", i+1, h.Action, h.Reasoning)
		}
	}

	tabsInfo := ""

	return fmt.Sprintf(`Задача: %s

Текущая страница:
- URL: %s
- Title: %s
- Description: %s
%s
Доступные элементы на странице:
%s

Текстовое содержимое (первые 1500 символов):
%s
%s

Что делать дальше?`,
		task,
		pageInfo.URL,
		pageInfo.Title,
		pageInfo.Description,
		tabsInfo,
		c.formatElements(pageInfo.Elements),
		truncateString(pageInfo.TextContent, 1500),
		historyText,
	)
}

// formatElements - formats page elements for prompt
func (c *openAIClient) formatElements(elements []entities.PageElement) string {
	if len(elements) == 0 {
		return "Нет доступных элементов"
	}

	var result string
	for i, el := range elements {
		if i >= 20 {
			result += fmt.Sprintf("\n... и ещё %d элементов\n", len(elements)-i)
			break
		}
		result += fmt.Sprintf("- %s: %s (селектор: %s, видимый: %v, кликабельный: %v)\n",
			el.Type, el.Text, el.Selector, el.IsVisible, el.IsClickable)
	}
	return result
}

// getFunctionDefinitions - returns function definitions for OpenAI
func (c *openAIClient) getFunctionDefinitions() []functionDef {
	return []functionDef{
		{
			Name:        "navigate",
			Description: "Перейти на указанный URL",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL для перехода",
					},
				},
				"required": []string{"url"},
			},
		},
		{
			Name:        "click",
			Description: "Кликнуть на элемент по CSS селектору",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS селектор элемента",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Почему ты кликаешь на этот элемент",
					},
				},
				"required": []string{"selector", "reasoning"},
			},
		},
		{
			Name:        "type",
			Description: "Ввести текст в поле ввода",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS селектор поля ввода",
					},
					"text": map[string]interface{}{
						"type":        "string",
						"description": "Текст для ввода",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Почему ты вводишь этот текст",
					},
				},
				"required": []string{"selector", "text", "reasoning"},
			},
		},
		{
			Name:        "wait_for_element",
			Description: "Подождать появления элемента на странице",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"selector": map[string]interface{}{
						"type":        "string",
						"description": "CSS селектор элемента",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Зачем ждать этот элемент",
					},
				},
				"required": []string{"selector", "reasoning"},
			},
		},
		{
			Name:        "ask_confirmation",
			Description: "Спросить подтверждение перед деструктивным действием",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Действие, которое нужно выполнить",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Почему это действие нужно выполнить",
					},
					"parameters": map[string]interface{}{
						"type":        "object",
						"description": "Параметры действия",
					},
				},
				"required": []string{"action", "reasoning", "parameters"},
			},
		},
		{
			Name:        "open_new_tab",
			Description: "Открыть новую вкладку и перейти на URL (если url пустой, просто открыть новую вкладку)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "URL для перехода в новой вкладке (опционально, можно оставить пустым)",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Почему нужно открыть новую вкладку",
					},
				},
				"required": []string{"reasoning"},
			},
		},
		{
			Name:        "switch_to_tab",
			Description: "Переключиться на вкладку по индексу (0 - первая вкладка, 1 - вторая и т.д.)",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"index": map[string]interface{}{
						"type":        "integer",
						"description": "Индекс вкладки (0 - первая вкладка)",
					},
					"reasoning": map[string]interface{}{
						"type":        "string",
						"description": "Почему нужно переключиться на эту вкладку",
					},
				},
				"required": []string{"index", "reasoning"},
			},
		},
		{
			Name:        "complete_task",
			Description: "Задача выполнена",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "Краткое описание выполненной работы",
					},
				},
				"required": []string{"summary"},
			},
		},
	}
}

// parseFunctionCall - parses function call from OpenAI response
func (c *openAIClient) parseFunctionCall(fc *functionCall) (entities.AgentResponse, error) {
	response := entities.AgentResponse{
		Action:     fc.Name,
		Parameters: fc.Arguments,
	}

	if reasoning, ok := fc.Arguments["reasoning"].(string); ok {
		response.Reasoning = reasoning
	}

	switch fc.Name {
	case "navigate":
		response.Action = "navigate"
	case "open_new_tab":
		response.Action = "open_new_tab"
	case "switch_to_tab":
		response.Action = "switch_to_tab"
	case "click":
		response.Action = "click"
	case "type":
		response.Action = "type"
	case "wait_for_element":
		response.Action = "wait_for_element"
	case "ask_confirmation":
		response.Action = "ask_confirmation"
		response.NeedsInput = true
		if action, ok := fc.Arguments["action"].(string); ok {
			response.UserMessage = fmt.Sprintf("Требуется подтверждение: %s\nПричина: %s", action, response.Reasoning)
		}
	case "complete_task":
		response.Action = "complete"
		response.IsComplete = true
		if summary, ok := fc.Arguments["summary"].(string); ok {
			response.UserMessage = fmt.Sprintf("Задача выполнена: %s", summary)
		}
	}

	return response, nil
}

// makeRequest - makes HTTP request to OpenAI API
func (c *openAIClient) makeRequest(ctx context.Context, req chatRequest) (*chatResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// isTaskComplete - checks if task is complete from message
func (c *openAIClient) isTaskComplete(message string) bool {
	completeKeywords := []string{"выполнено", "готово", "завершено", "сделано", "done", "complete", "finished"}
	lowerMessage := strings.ToLower(message)
	for _, keyword := range completeKeywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}

// truncateString - truncates string to maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
