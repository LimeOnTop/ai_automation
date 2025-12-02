package terminal

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"ai_automation/application/agent"
	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"
	"ai_automation/infrastructure/ai"
	"ai_automation/infrastructure/browser"
	"ai_automation/infrastructure/security"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type TerminalInterface struct {
	agent       *agent.Agent
	browserCtrl interfaces.BrowserController
	logger      *logrus.Logger
	reader      *bufio.Reader
}

func NewTerminalInterface() (*TerminalInterface, error) {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// .env file is optional
		fmt.Println("Warning: .env file not found, using environment variables")
	}

	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Initialize browser controller
	browserCtrl, err := browser.NewSeleniumController(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize browser: %w", err)
	}

	// Initialize AI service
	aiService, err := ai.NewOpenAIClient(logger)
	if err != nil {
		browserCtrl.Close()
		return nil, fmt.Errorf("failed to initialize AI service: %w", err)
	}

	// Initialize security layer
	securityLayer := security.NewSecurityLayer(logger)

	// Initialize agent
	ag := agent.NewAgent(browserCtrl, aiService, securityLayer, logger)

	return &TerminalInterface{
		agent:       ag,
		browserCtrl: browserCtrl,
		logger:      logger,
		reader:      bufio.NewReader(os.Stdin),
	}, nil
}

func (t *TerminalInterface) Run() error {
	defer t.browserCtrl.Close()

	fmt.Println("AI Браузер Агент")
	fmt.Println("=================")
	fmt.Println("Введите задачу для агента, или 'quit' для выхода")
	fmt.Println()

	for {
		fmt.Print("> ")
		input, err := t.reader.ReadString('\n')
		if err != nil {
			return err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" || input == "q" {
			fmt.Println("До свидания!")
			return nil
		}

		// Create task
		task := &entities.Task{
			ID:          fmt.Sprintf("task-%d", len(input)),
			Description: input,
			Status:      entities.TaskStatusPending,
		}

		// Execute task
		fmt.Printf("\nНачинаю выполнение задачи: %s\n\n", task.Description)
		
		ctx := context.Background()
		err = t.agent.ExecuteTask(ctx, task, t.reader)
		
		if err != nil {
			if task.Status == entities.TaskStatusWaiting {
				// Task is waiting for user input, continue loop
				continue
			} else {
				fmt.Printf("\nЗадача не выполнена: %v\n\n", err)
			}
		} else {
			fmt.Printf("\nЗадача выполнена\n\n")
		}
	}
}

func (t *TerminalInterface) Close() error {
	return t.browserCtrl.Close()
}

