package terminal

import (
	"ai_automation/application/agent"
	"ai_automation/domain/entities"
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

type TerminalInterface struct {
	agent *agent.Agent
}

// NewTerminalInterface - creates new terminal interface
func NewTerminalInterface(agent *agent.Agent) *TerminalInterface {
	return &TerminalInterface{
		agent: agent,
	}
}

// Run - runs interactive terminal interface
func (t *TerminalInterface) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("=" + strings.Repeat("=", 60) + "=")
	fmt.Println("  AI Browser Automation Agent")
	fmt.Println("=" + strings.Repeat("=", 60) + "=")
	fmt.Println()
	fmt.Println("Введите задачу для агента (или 'exit' для выхода)")
	fmt.Println("Пример: Найди 3 подходящие вакансии AI-инженера на hh.ru")
	fmt.Println()

	for {

		select {
		case <-ctx.Done():
			fmt.Println("\nПолучен сигнал завершения...")
			return ctx.Err()
		default:
		}

		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		task := strings.TrimSpace(scanner.Text())
		if task == "" {
			continue
		}

		if task == "exit" || task == "quit" {
			fmt.Println("До свидания!")
			return nil
		}

		if err := t.executeTask(ctx, task); err != nil {
			if err.Error() == "requires user input" {

				if err := t.handleUserInput(ctx, scanner); err != nil {
					fmt.Printf("Ошибка: %v\n", err)
				}
			} else if err.Error() == "requires confirmation" {

				if err := t.handleConfirmation(ctx, scanner); err != nil {
					fmt.Printf("Ошибка: %v\n", err)
				}
			} else {
				fmt.Printf("Ошибка выполнения задачи: %v\n", err)
			}
		}

		fmt.Println()
		fmt.Println(strings.Repeat("-", 62))
		fmt.Println()
	}

	return scanner.Err()
}

// executeTask - executes task through agent
func (t *TerminalInterface) executeTask(ctx context.Context, task string) error {
	return t.agent.ExecuteTask(ctx, task)
}

// handleUserInput - handles user input from terminal
func (t *TerminalInterface) handleUserInput(ctx context.Context, scanner *bufio.Scanner) error {
	fmt.Println("\nАгент запрашивает дополнительную информацию.")
	fmt.Print("Введите ответ (или 'continue' чтобы продолжить): ")

	if !scanner.Scan() {
		return fmt.Errorf("failed to read input")
	}

	answer := strings.TrimSpace(scanner.Text())
	if answer == "continue" {

		return t.agent.ExecuteTask(ctx, t.agent.GetTask())
	}

	fmt.Printf("Получен ответ: %s\n", answer)
	return nil
}

// handleConfirmation - handles user confirmation for destructive actions
func (t *TerminalInterface) handleConfirmation(ctx context.Context, scanner *bufio.Scanner) error {
	pendingAction := t.agent.GetPendingAction()
	if pendingAction == nil {
		return fmt.Errorf("no pending action found")
	}

	fmt.Println("\nТРЕБУЕТСЯ ПОДТВЕРЖДЕНИЕ")
	fmt.Printf("Действие: %s\n", pendingAction.Action)
	fmt.Printf("Причина: %s\n", pendingAction.Reasoning)
	fmt.Print("\nПодтвердить? (yes/no): ")

	if !scanner.Scan() {
		return fmt.Errorf("failed to read confirmation")
	}

	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer == "yes" || answer == "y" || answer == "да" {

		response := entities.AgentResponse{
			Action:     pendingAction.Action,
			Reasoning:  pendingAction.Reasoning,
			Parameters: pendingAction.Parameters,
		}

		if err := t.agent.ExecuteActionWithConfirmation(ctx, response); err != nil {
			return fmt.Errorf("failed to execute action: %w", err)
		}

		return t.agent.ExecuteTask(ctx, t.agent.GetTask())
	}

	fmt.Println("Действие отменено пользователем")
	return nil
}
