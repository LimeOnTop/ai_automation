package interfaces

import (
	"ai_automation/domain/entities"
	"context"
)

// BrowserController defines the interface for browser automation
type BrowserController interface {
	// Navigate navigates to a URL
	Navigate(ctx context.Context, url string) error
	
	// Click clicks on an element by selector
	Click(ctx context.Context, selector string) error
	
	// TypeText types text into an element
	TypeText(ctx context.Context, selector string, text string) error
	
	// ExtractPageInfo extracts structured information from the current page
	ExtractPageInfo(ctx context.Context) (*entities.PageInfo, error)
	
	// Wait waits for a condition or time
	Wait(ctx context.Context, condition string, timeout int) error
	
	// Scroll scrolls the page
	Scroll(ctx context.Context, direction string, amount int) error
	
	// GetCurrentURL returns the current page URL
	GetCurrentURL(ctx context.Context) (string, error)
	
	// GetPageTitle returns the current page title
	GetPageTitle(ctx context.Context) (string, error)
	
	// TakeScreenshot takes a screenshot
	TakeScreenshot(ctx context.Context) ([]byte, error)
	
	// Close closes the browser
	Close() error
	
	// IsElementVisible checks if an element is visible
	IsElementVisible(ctx context.Context, selector string) (bool, error)
	
	// FindElementsByText finds elements containing specific text
	FindElementsByText(ctx context.Context, text string) ([]entities.PageElement, error)
}

