package main

import (
	"ai_automation/application/agent"
	"ai_automation/infrastructure/ai"
	"ai_automation/infrastructure/browser"
	"ai_automation/infrastructure/security"
	"ai_automation/presentation/terminal"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Загружаем переменные окружения
	if err := godotenv.Load(); err != nil {
		// Не критично, если .env файла нет
		fmt.Println(".env файл не найден, используем переменные окружения системы")
	}

	// Проверяем наличие API ключа
	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Fatal("OPENAI_API_KEY не установлен. Установите переменную окружения OPENAI_API_KEY")
	}

	// Создаём контекст с обработкой сигналов
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка сигналов для корректного завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Канал для отслеживания завершения терминального интерфейса
	done := make(chan error, 1)

	// Горутина для обработки сигналов
	go func() {
		sig := <-sigChan
		fmt.Printf("\n\nПолучен сигнал %v, начинаю graceful shutdown...\n", sig)
		cancel()
	}()

	// Инициализируем компоненты
	fmt.Println("Инициализация компонентов...")

	// AI клиент
	aiClient, err := ai.NewOpenAIClient()
	if err != nil {
		log.Fatalf("Ошибка создания AI клиента: %v", err)
	}

	// Браузер
	browserController, err := browser.NewBrowserController()
	if err != nil {
		log.Fatalf("Ошибка создания браузера: %v", err)
	}

	// Функция для корректного закрытия браузера
	closeBrowser := func() {
		fmt.Println("Сохранение состояния браузера...")
		if err := browserController.Close(); err != nil {
			fmt.Printf("Ошибка при закрытии браузера: %v\n", err)
		} else {
			fmt.Println("Состояние браузера сохранено")
		}
	}
	defer closeBrowser()

	// Security layer
	securityLayer := security.NewSecurityLayer()

	// Агент
	agent := agent.NewAgent(aiClient, browserController, securityLayer)

	// Терминальный интерфейс
	terminalInterface := terminal.NewTerminalInterface(agent)

	fmt.Println("Все компоненты инициализированы")
	fmt.Println()

	// Запускаем терминальный интерфейс в отдельной горутине
	go func() {
		done <- terminalInterface.Run(ctx)
	}()

	// Ждём завершения или сигнала
	select {
	case err = <-done:
		// Терминальный интерфейс завершился
		if err != nil && err.Error() != "context canceled" && ctx.Err() == nil {
			log.Printf("Ошибка выполнения: %v", err)
		}
	case <-ctx.Done():
		// Получен сигнал завершения
		fmt.Println("Ожидание завершения операций...")
		// Сохраняем состояние браузера перед завершением
		if browserController != nil {
			if err := browserController.SaveState(); err == nil {
				fmt.Println("Состояние браузера сохранено")
			}
		}
		// Даём время на завершение (максимум 5 секунд)
		select {
		case err = <-done:
			if err != nil && err.Error() != "context canceled" {
				fmt.Printf("Ошибка при завершении: %v\n", err)
			}
		case <-time.After(5 * time.Second):
			fmt.Println("Таймаут ожидания завершения операций")
		}
	}

	fmt.Println("\nПрограмма завершена")
}
