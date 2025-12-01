package interfaces

import (
	"ai_automation/domain/entities"
	"context"
)

// Browser представляет интерфейс для управления браузером
type Browser interface {
	// Navigate переходит по URL
	Navigate(ctx context.Context, url string) error
	
	// GetPageInfo получает информацию о текущей странице
	GetPageInfo(ctx context.Context) (entities.PageInfo, error)
	
	// Click кликает на элемент по селектору
	Click(ctx context.Context, selector string) error
	
	// Type вводит текст в элемент
	Type(ctx context.Context, selector string, text string) error
	
	// WaitForElement ждёт появления элемента
	WaitForElement(ctx context.Context, selector string) error
	
	// GetElements получает все интерактивные элементы на странице
	GetElements(ctx context.Context) ([]entities.PageElement, error)
	
	// GetTextContent получает текстовое содержимое страницы
	GetTextContent(ctx context.Context) (string, error)
	
	// Screenshot делает скриншот страницы
	Screenshot(ctx context.Context, path string) error
	
	// SaveState сохраняет состояние браузера (cookies, localStorage)
	SaveState() error
	
	// OpenNewTab открывает новую вкладку и переключается на неё
	OpenNewTab(ctx context.Context, url string) error
	
	// SwitchToTab переключается на вкладку по индексу (0 - первая вкладка)
	SwitchToTab(index int) error
	
	// GetTabsCount возвращает количество открытых вкладок
	GetTabsCount() int
	
	// GetCurrentTabIndex возвращает индекс текущей активной вкладки
	GetCurrentTabIndex() int
	
	// Close закрывает браузер
	Close() error
}
