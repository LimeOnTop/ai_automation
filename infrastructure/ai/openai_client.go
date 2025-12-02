package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"

	"github.com/sirupsen/logrus"
)

type OpenAIClient struct {
	apiKey string
	client *http.Client
	logger *logrus.Logger
	model  string
}

func NewOpenAIClient(logger *logrus.Logger) (*OpenAIClient, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is not set")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o" // Use GPT-4o by default
	}

	return &OpenAIClient{
		apiKey: apiKey,
		client: &http.Client{},
		logger: logger,
		model:  model,
	}, nil
}

func (c *OpenAIClient) DecideNextAction(ctx context.Context, task *entities.Task, pageInfo *entities.PageInfo, history []entities.Action) (*entities.Action, error) {
	contextSummary := c.buildContextSummary(pageInfo, history)
	historySummary := c.formatHistorySummary(history)

	// Check if extract was used recently - if so, don't allow it again
	hasRecentExtract := false
	for i := len(history) - 1; i >= 0 && i >= len(history)-3; i-- {
		if history[i].Type == entities.ActionExtract {
			hasRecentExtract = true
			break
		}
	}

	// Check if scroll was used too many times recently - if so, disable it temporarily
	hasRecentScrolls := false
	scrollCount := 0
	for i := len(history) - 1; i >= 0 && i >= len(history)-5; i-- {
		if history[i].Type == entities.ActionScroll {
			scrollCount++
		}
	}
	if scrollCount >= 3 {
		hasRecentScrolls = true
	}

	tools := c.buildTools()
	if hasRecentExtract {
		// Remove extract tool if it was used recently
		filteredTools := []Tool{}
		for _, tool := range tools {
			if tool.Function.Name != "extract" {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
	}
	if hasRecentScrolls {
		// Remove scroll tool if it was used too many times
		filteredTools := []Tool{}
		for _, tool := range tools {
			if tool.Function.Name != "scroll" {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
	}

	prompt := c.buildDecisionPrompt(task, contextSummary, pageInfo, historySummary, hasRecentExtract, hasRecentScrolls)

	response, err := c.callAPI(ctx, prompt, tools)
	if err != nil {
		return nil, err
	}

	// Check if response indicates task completion
	if response == "" || response == "null" || strings.Contains(strings.ToLower(response), "task complete") ||
		strings.Contains(strings.ToLower(response), "задача выполнена") {
		return nil, nil
	}

	action, err := c.parseActionResponse(response)
	if err != nil {
		return nil, err
	}

	return action, nil
}

func (c *OpenAIClient) AnalyzePage(ctx context.Context, pageInfo *entities.PageInfo, task *entities.Task) (string, error) {
	prompt := fmt.Sprintf(`Analyze this web page and provide a brief summary relevant to the task: "%s"

Page URL: %s
Page Title: %s
Description: %s

Visible Elements: %d
Links: %d
Forms: %d
Buttons: %d

Key visible text (first 500 chars): %s

Provide a concise analysis focusing on elements that might help complete the task.`,
		task.Description,
		pageInfo.URL,
		pageInfo.Title,
		pageInfo.Description,
		len(pageInfo.Elements),
		len(pageInfo.Links),
		len(pageInfo.Forms),
		len(pageInfo.Buttons),
		c.truncateText(pageInfo.TextContent, 500),
	)

	response, err := c.callAPI(ctx, prompt, nil)
	if err != nil {
		return "", err
	}

	return response, nil
}

// Helper methods

func (c *OpenAIClient) buildDecisionPrompt(task *entities.Task, contextSummary string, pageInfo *entities.PageInfo, historySummary string, extractDisabled bool, scrollDisabled bool) string {
	extractWarning := ""
	if extractDisabled {
		extractWarning = "\nWARNING: Extract action was recently used and is now disabled. You MUST use click or type_text actions with the elements listed below.\n"
	}

	scrollWarning := ""
	if scrollDisabled {
		scrollWarning = "\nWARNING: Scroll action was used too many times recently and is now disabled. You MUST click on elements from the list above. The browser will automatically scroll to elements when you click them.\n"
	}

	warnings := extractWarning + scrollWarning

	elementsInfo := c.formatPageElements(pageInfo)
	if elementsInfo == "Интерактивные элементы не найдены" {
		elementsInfo = "Попробуйте прокрутить страницу или использовать поиск по тексту элементов"
	}

	return fmt.Sprintf(`You are an autonomous AI agent that controls a web browser to complete user tasks.

Current Task: "%s"

Current Page Context:
- URL: %s
- Title: %s
- %s
%sAvailable interactive elements on the page:
%s

History of actions: %s

Based on the task, current page state, and action history, decide what action to take next.

CRITICAL INSTRUCTIONS:
1. Look at the visible text above - it shows what's actually on the page
2. The page ALWAYS has interactive elements. All elements are listed above, even if they're not currently visible - the browser will scroll to them automatically when you click.
3. You MUST use click actions on elements from the list above. Use the selectors provided.
4. Click on elements that contain text relevant to your task
5. Look for buttons or icons that might perform actions you need
6. Use XPath to find elements by text if selector doesn't work: //tr[contains(text(), 'текст')] or //li[contains(text(), 'текст')]
7. DO NOT use extract - use click on the elements listed above
8. DO NOT scroll repeatedly - scroll is only for initial page exploration. After scrolling once or twice, you MUST click on elements.
9. All actions are equal - choose the one that best fits your current task state
10. Return null only if task is complete

Respond with a JSON object containing the action to take, or return null/empty if task is complete.`,
		task.Description,
		pageInfo.URL,
		pageInfo.Title,
		contextSummary,
		warnings,
		elementsInfo,
		historySummary,
	)
}

func (c *OpenAIClient) buildTools() []Tool {
	return []Tool{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "navigate",
				Description: "Navigate to a URL",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to navigate to",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Why you are navigating to this URL",
						},
					},
					"required": []string{"url", "description"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "click",
				Description: "Click on an element",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"selector": map[string]interface{}{
							"type":        "string",
							"description": "CSS selector, XPath, or text to identify the element",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "What you are clicking and why",
						},
					},
					"required": []string{"selector", "description"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "type_text",
				Description: "Type text into an input field",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"selector": map[string]interface{}{
							"type":        "string",
							"description": "CSS selector or XPath to identify the input field",
						},
						"text": map[string]interface{}{
							"type":        "string",
							"description": "The text to type",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "What you are typing and why",
						},
					},
					"required": []string{"selector", "text", "description"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "scroll",
				Description: "Scroll the page",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"direction": map[string]interface{}{
							"type":        "string",
							"description": "Direction: 'down' or 'up'",
						},
						"amount": map[string]interface{}{
							"type":        "integer",
							"description": "Pixels to scroll",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Why you are scrolling",
						},
					},
					"required": []string{"direction", "description"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "extract",
				Description: "Extract more detailed information from the page. Use ONLY when you cannot find the element you need to click or type into. Prefer using click and type_text actions instead.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"description": map[string]interface{}{
							"type":        "string",
							"description": "What specific information you need to extract",
						},
					},
					"required": []string{"description"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "wait",
				Description: "Wait for a condition or time",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"timeout": map[string]interface{}{
							"type":        "integer",
							"description": "Seconds to wait",
						},
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Why you are waiting",
						},
					},
					"required": []string{"timeout", "description"},
				},
			},
		},
	}
}

func (c *OpenAIClient) callAPI(ctx context.Context, prompt string, tools []Tool) (string, error) {
	messages := []Message{
		{
			Role:    "system",
			Content: "You are an autonomous AI agent that controls a web browser. You must make decisions based on the current page state and task requirements. Always respond with valid JSON when using tools.",
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	requestBody := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"temperature": 0.7,
	}

	if len(tools) > 0 {
		requestBody["tools"] = tools
		requestBody["tool_choice"] = "auto"
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var apiResponse APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", err
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	choice := apiResponse.Choices[0]

	// Handle tool calls
	if len(choice.Message.ToolCalls) > 0 {
		toolCall := choice.Message.ToolCalls[0]
		// Parse arguments JSON string
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return "", fmt.Errorf("failed to parse tool call arguments: %w", err)
		}
		// Return the tool call as JSON
		toolCallJSON := map[string]interface{}{
			"name":      toolCall.Function.Name,
			"arguments": args,
		}
		jsonData, err := json.Marshal(toolCallJSON)
		if err != nil {
			return "", err
		}
		return string(jsonData), nil
	}

	return choice.Message.Content, nil
}

func (c *OpenAIClient) parseActionResponse(response string) (*entities.Action, error) {
	// Extract JSON from markdown code blocks if present
	cleanedResponse := c.extractJSONFromMarkdown(response)

	// Try to parse as tool call first
	var toolCall struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}

	if err := json.Unmarshal([]byte(cleanedResponse), &toolCall); err == nil && toolCall.Name != "" {
		action := &entities.Action{}

		switch toolCall.Name {
		case "navigate":
			action.Type = entities.ActionNavigate
			if url, ok := toolCall.Arguments["url"].(string); ok {
				action.URL = url
			}
		case "click":
			action.Type = entities.ActionClick
			if selector, ok := toolCall.Arguments["selector"].(string); ok {
				action.Selector = selector
			}
		case "type_text":
			action.Type = entities.ActionTypeText
			if selector, ok := toolCall.Arguments["selector"].(string); ok {
				action.Selector = selector
			}
			if text, ok := toolCall.Arguments["text"].(string); ok {
				action.Text = text
			}
		case "scroll":
			action.Type = entities.ActionScroll
			// Scroll direction and amount can be handled in the action execution
		case "extract":
			action.Type = entities.ActionExtract
		case "wait":
			action.Type = entities.ActionWait
		default:
			return nil, fmt.Errorf("unknown action type: %s", toolCall.Name)
		}

		if desc, ok := toolCall.Arguments["description"].(string); ok {
			action.Description = desc
		}

		return action, nil
	}

	// Try to parse as direct JSON action
	var directAction map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedResponse), &directAction); err == nil {
		return c.mapToAction(directAction), nil
	}

	return nil, fmt.Errorf("failed to parse response: %s", response)
}

// extractJSONFromMarkdown - extracts JSON from markdown code blocks
func (c *OpenAIClient) extractJSONFromMarkdown(text string) string {
	// Remove markdown code block markers
	text = strings.TrimSpace(text)

	// Check for ```json or ``` blocks
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		var jsonLines []string
		inCodeBlock := false

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				if !inCodeBlock {
					inCodeBlock = true
					continue
				} else {
					break
				}
			}
			if inCodeBlock {
				jsonLines = append(jsonLines, line)
			}
		}

		if len(jsonLines) > 0 {
			return strings.Join(jsonLines, "\n")
		}
	}

	// Try to find JSON object in the text
	startIdx := strings.Index(text, "{")
	endIdx := strings.LastIndex(text, "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		return text[startIdx : endIdx+1]
	}

	return text
}

func (c *OpenAIClient) mapToAction(data map[string]interface{}) *entities.Action {
	action := &entities.Action{}

	if actionType, ok := data["type"].(string); ok {
		action.Type = entities.ActionType(actionType)
	}
	if selector, ok := data["selector"].(string); ok {
		action.Selector = selector
	}
	if text, ok := data["text"].(string); ok {
		action.Text = text
	}
	if url, ok := data["url"].(string); ok {
		action.URL = url
	}
	if desc, ok := data["description"].(string); ok {
		action.Description = desc
	}

	return action
}

func (c *OpenAIClient) buildContextSummary(pageInfo *entities.PageInfo, history []entities.Action) string {
	parts := []string{}

	if len(pageInfo.Links) > 0 {
		parts = append(parts, fmt.Sprintf("%d links available", len(pageInfo.Links)))
	}
	if len(pageInfo.Forms) > 0 {
		parts = append(parts, fmt.Sprintf("%d forms available", len(pageInfo.Forms)))
	}
	if len(pageInfo.Buttons) > 0 {
		parts = append(parts, fmt.Sprintf("%d buttons available", len(pageInfo.Buttons)))
	}

	return strings.Join(parts, ", ")
}

func (c *OpenAIClient) formatPageElements(pageInfo *entities.PageInfo) string {
	var builder strings.Builder

	// Show visible text content first (helps AI understand page context)
	if pageInfo.TextContent != "" {
		textPreview := c.truncateText(pageInfo.TextContent, 500)
		if len(textPreview) > 0 {
			builder.WriteString("Видимый текст на странице (первые 500 символов):\n")
			builder.WriteString(textPreview)
			builder.WriteString("\n\n")
		}
	}

	// Format buttons
	if len(pageInfo.Buttons) > 0 {
		builder.WriteString("Кнопки:\n")
		for i, btn := range pageInfo.Buttons {
			if i >= 50 {
				break
			}
			if btn.Text != "" {
				builder.WriteString(fmt.Sprintf("  - \"%s\" (селектор: %s)\n", c.truncateText(btn.Text, 100), btn.Selector))
			}
		}
		builder.WriteString("\n")
	}

	// Format links
	if len(pageInfo.Links) > 0 {
		builder.WriteString("Ссылки:\n")
		for i, link := range pageInfo.Links {
			if i >= 60 {
				break
			}
			if link.Text != "" {
				selector := link.Selector
				if selector == "" {
					selector = fmt.Sprintf("a:contains('%s')", c.truncateText(link.Text, 50))
				}
				builder.WriteString(fmt.Sprintf("  - \"%s\" (селектор: %s)\n", c.truncateText(link.Text, 100), selector))
			}
		}
		builder.WriteString("\n")
	}

	// Format interactive elements (list items, table rows, etc.)
	if len(pageInfo.Elements) > 0 {
		builder.WriteString("Интерактивные элементы:\n")
		count := 0
		for _, elem := range pageInfo.Elements {
			if !elem.IsClickable {
				continue
			}
			if count >= 80 {
				break
			}
			text := elem.Text
			if text == "" {
				text = "без текста"
			}
			maxTextLen := 120
			if elem.TagName == "tr" || elem.TagName == "li" {
				maxTextLen = 150
			}
			builder.WriteString(fmt.Sprintf("  - %s: \"%s\" (селектор: %s)\n", elem.TagName, c.truncateText(text, maxTextLen), elem.Selector))
			count++
		}
		builder.WriteString("\n")
	}

	// Format forms and inputs
	if len(pageInfo.Forms) > 0 {
		builder.WriteString("Формы и поля ввода:\n")
		for i, form := range pageInfo.Forms {
			if i >= 5 {
				break
			}
			builder.WriteString(fmt.Sprintf("  Форма (метод: %s, действие: %s):\n", form.Method, form.Action))
			for _, input := range form.Inputs {
				label := input.Label
				if label == "" {
					label = input.Placeholder
				}
				if label == "" {
					label = input.Name
				}
				builder.WriteString(fmt.Sprintf("    - Поле \"%s\" (тип: %s, имя: %s)\n", label, input.Type, input.Name))
			}
		}
		builder.WriteString("\n")
	}

	if builder.Len() == 0 {
		return "Интерактивные элементы не найдены. Попробуйте прокрутить страницу."
	}

	return builder.String()
}

func (c *OpenAIClient) formatHistorySummary(history []entities.Action) string {
	if len(history) == 0 {
		return "Нет выполненных действий"
	}

	var parts []string
	for i, action := range history {
		desc := getActionTypeDescription(action.Type)
		if action.Description != "" {
			desc += ": " + action.Description
		}
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, desc))
	}

	return strings.Join(parts, "\n")
}

func getActionTypeDescription(actionType entities.ActionType) string {
	switch actionType {
	case entities.ActionNavigate:
		return "Переход на страницу"
	case entities.ActionClick:
		return "Клик"
	case entities.ActionTypeText:
		return "Ввод текста"
	case entities.ActionScroll:
		return "Прокрутка"
	case entities.ActionExtract:
		return "Извлечение информации"
	case entities.ActionWait:
		return "Ожидание"
	default:
		return string(actionType)
	}
}

func (c *OpenAIClient) truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

// API structures

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type APIResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
}

// Ensure OpenAIClient implements AIService interface
var _ interfaces.AIService = (*OpenAIClient)(nil)
