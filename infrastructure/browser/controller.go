package browser

import (
	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

type browserController struct {
	browser     playwright.Browser
	page        playwright.Page
	context     playwright.BrowserContext
	storagePath string
	pages       []playwright.Page
	pagesMutex  sync.Mutex
}

const browserStateDir = ".browser_state"
const browserStateFile = "state.json"

// NewBrowserController - creates new browser controller
func NewBrowserController() (interfaces.Browser, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start playwright: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	stateDir := filepath.Join(homeDir, browserStateDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	storagePath := filepath.Join(stateDir, browserStateFile)

	contextOptions := playwright.BrowserNewContextOptions{
		Viewport: &playwright.Size{
			Width:  1280,
			Height: 720,
		},

		JavaScriptEnabled: playwright.Bool(true),

		IgnoreHttpsErrors: playwright.Bool(true),

		AcceptDownloads: playwright.Bool(true),

		BypassCSP: playwright.Bool(true),

		Permissions: []string{
			"geolocation",
			"notifications",
			"camera",
			"microphone",
			"clipboard-read",
			"clipboard-write",
		},

		Geolocation: &playwright.Geolocation{
			Latitude:  55.7558,
			Longitude: 37.6173,
		},

		UserAgent: playwright.String("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	}

	if _, err := os.Stat(storagePath); err == nil {

		data, err := os.ReadFile(storagePath)
		if err == nil {
			var storageState playwright.StorageState
			if err := json.Unmarshal(data, &storageState); err == nil {
				contextOptions.StorageState = storageState.ToOptionalStorageState()
			}
		}
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false),
		SlowMo:   playwright.Float(100),
		Args: []string{
			"--disable-popup-blocking",
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-web-security",
			"--disable-features=IsolateOrigins,site-per-process",
			"--allow-running-insecure-content",
			"--disable-infobars",
			"--disable-notifications",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	context, err := browser.NewContext(contextOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	page, err := context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}

	context.GrantPermissions([]string{
		"geolocation",
		"notifications",
		"camera",
		"microphone",
		"clipboard-read",
		"clipboard-write",
		"payment-handler",
		"persistent-storage",
	}, playwright.BrowserContextGrantPermissionsOptions{
		Origin: playwright.String("*"),
	})

	controller := &browserController{
		browser:     browser,
		page:        page,
		context:     context,
		storagePath: storagePath,
		pages:       []playwright.Page{page},
	}

	page.OnDialog(func(dialog playwright.Dialog) {

		dialog.Accept()
	})

	context.OnPage(func(newPage playwright.Page) {
		controller.pagesMutex.Lock()
		defer controller.pagesMutex.Unlock()

		controller.pages = append(controller.pages, newPage)

		controller.page = newPage

		newPage.OnDialog(func(dialog playwright.Dialog) {

			dialog.Accept()
		})

		newPage.OnClose(func(closedPage playwright.Page) {
			controller.pagesMutex.Lock()
			defer controller.pagesMutex.Unlock()

			for i, p := range controller.pages {
				if p == closedPage {
					controller.pages = append(controller.pages[:i], controller.pages[i+1:]...)
					break
				}
			}

			if controller.page == closedPage && len(controller.pages) > 0 {
				controller.page = controller.pages[0]
			}
		})
	})

	return controller, nil
}

// Navigate - navigates to the specified URL
func (b *browserController) Navigate(ctx context.Context, url string) error {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	_, err := currentPage.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	})
	return err
}

// GetPageInfo - gets information about the current page
func (b *browserController) GetPageInfo(ctx context.Context) (entities.PageInfo, error) {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	currentPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(3000),
	})

	url := currentPage.URL()
	title, _ := currentPage.Title()

	description, _ := currentPage.Locator("meta[name='description']").GetAttribute("content")
	if description == "" {

		text, _ := b.GetTextContent(ctx)
		description = truncateString(text, 500)
	}

	elements, err := b.GetElements(ctx)
	if err != nil {
		return entities.PageInfo{}, err
	}

	textContent, _ := b.GetTextContent(ctx)

	return entities.PageInfo{
		URL:         url,
		Title:       title,
		Description: description,
		Elements:    elements,
		TextContent: textContent,
	}, nil
}

// Click - clicks on an element by CSS selector
func (b *browserController) Click(ctx context.Context, selector string) error {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	locator := currentPage.Locator(selector)

	err := locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return fmt.Errorf("element not found or not visible: %w", err)
	}

	if err := locator.Click(); err != nil {
		return err
	}

	currentPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(5000),
	})

	time.Sleep(300 * time.Millisecond)

	return nil
}

// Type - types text into an input field
func (b *browserController) Type(ctx context.Context, selector string, text string) error {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	locator := currentPage.Locator(selector)

	err := locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(5000),
	})
	if err != nil {
		return fmt.Errorf("input field not found: %w", err)
	}

	locator.Clear()
	if err := locator.Fill(text); err != nil {
		return err
	}

	time.Sleep(200 * time.Millisecond)

	return nil
}

// WaitForElement - waits for an element to appear on the page
func (b *browserController) WaitForElement(ctx context.Context, selector string) error {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	currentPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(5000),
	})

	locator := currentPage.Locator(selector)
	err := locator.WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(20000),
	})

	if err != nil {

		elements, _ := b.GetElements(ctx)
		var similarElements []string
		var inputElements []string
		selectorLower := strings.ToLower(selector)

		for _, el := range elements {
			elSelectorLower := strings.ToLower(el.Selector)
			if strings.Contains(elSelectorLower, selectorLower) ||
				strings.Contains(selectorLower, elSelectorLower) {
				similarElements = append(similarElements, fmt.Sprintf("%s (%s)", el.Selector, el.Text))
			}

			if el.Type == "input" || el.Type == "textarea" {
				text := el.Text
				if text == "" {
					text = "без текста"
				}
				inputElements = append(inputElements, fmt.Sprintf("%s [%s]", el.Selector, text))
			}
		}

		var errorParts []string
		errorParts = append(errorParts, fmt.Sprintf("element '%s' not found after 20s timeout", selector))

		if len(inputElements) > 0 {
			errorParts = append(errorParts, fmt.Sprintf("available input elements on page: %s", strings.Join(inputElements[:min(10, len(inputElements))], ", ")))
		}

		if len(similarElements) > 0 {
			errorParts = append(errorParts, fmt.Sprintf("similar elements found: %s", strings.Join(similarElements[:min(5, len(similarElements))], ", ")))
		} else {
			errorParts = append(errorParts, "element may not exist on this page or page structure has changed")
		}

		return fmt.Errorf("%s", strings.Join(errorParts, ". "))
	}

	return nil
}

// min - returns minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetElements - extracts all interactive elements from the page
func (b *browserController) GetElements(ctx context.Context) ([]entities.PageElement, error) {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	jsCode := `
	() => {
		const elements = [];
		const selectors = [
			'button', 'a', 'input', 'select', 'textarea',
			'[role="button"]', '[onclick]', '[data-testid]',
			'[class*="button"]', '[class*="btn"]', '[class*="link"]',
			'[data-qa]', '[data-testid]', '[aria-label]'
		];
		

		const allInputs = document.querySelectorAll('input, textarea, select');
		allInputs.forEach(el => {
			const rect = el.getBoundingClientRect();
			const style = window.getComputedStyle(el);

			const isVisible = (rect.width > 0 && rect.height > 0) || 
				(style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0');
			

			if (rect.width === 0 && rect.height === 0 && style.display === 'none') return;
			
			const text = el.value || el.placeholder || el.textContent?.trim() || '';
			const tagName = el.tagName.toLowerCase();
			

			let uniqueSelector = tagName;
			if (el.getAttribute('data-qa')) {
				uniqueSelector = '[data-qa="' + el.getAttribute('data-qa') + '"]';
			} else if (el.getAttribute('data-testid')) {
				uniqueSelector = '[data-testid="' + el.getAttribute('data-testid') + '"]';
			} else if (el.id) {
				uniqueSelector = '#' + el.id;
			} else if (el.getAttribute('name')) {
				uniqueSelector = tagName + '[name="' + el.getAttribute('name') + '"]';
			} else if (el.getAttribute('type')) {
				uniqueSelector = tagName + '[type="' + el.getAttribute('type') + '"]';
			} else if (el.className) {

				let classNameStr = '';
				if (typeof el.className === 'string') {
					classNameStr = el.className;
				} else if (el.className && typeof el.className.baseVal === 'string') {

					classNameStr = el.className.baseVal;
				} else if (el.className && el.className.toString) {
					classNameStr = el.className.toString();
				}
				
				if (classNameStr) {
					const classes = classNameStr.split(' ').filter(c => c && !c.includes(' ')).slice(0, 2).join('.');
					if (classes) {
						uniqueSelector = tagName + '.' + classes;
					}
				}
			}
			
			const attributes = {};
			Array.from(el.attributes).forEach(attr => {
				if (attr.name.startsWith('data-') || attr.name === 'id' || attr.name === 'class' || attr.name === 'name' || attr.name === 'type') {
					attributes[attr.name] = attr.value;
				}
			});
			
			elements.push({
				type: tagName,
				selector: uniqueSelector,
				text: text.substring(0, 200),
				attributes: attributes,
				isVisible: isVisible,
				isClickable: el.disabled !== true,
				position: {
					x: Math.round(rect.left + rect.width / 2),
					y: Math.round(rect.top + rect.height / 2)
				}
			});
		});
		

		selectors.forEach(selector => {
			document.querySelectorAll(selector).forEach(el => {

				if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT') {
					return;
				}
				
				const rect = el.getBoundingClientRect();
				const isVisible = rect.width > 0 && rect.height > 0 && 
					window.getComputedStyle(el).display !== 'none' &&
					window.getComputedStyle(el).visibility !== 'hidden';
				
				if (!isVisible) return;
				
				const text = el.textContent?.trim() || el.value || el.placeholder || '';
				const tagName = el.tagName.toLowerCase();
				

				let uniqueSelector = selector;
				if (el.id) {
					uniqueSelector = '#' + el.id;
				} else if (el.className) {

					let classNameStr = '';
					if (typeof el.className === 'string') {
						classNameStr = el.className;
					} else if (el.className && typeof el.className.baseVal === 'string') {

						classNameStr = el.className.baseVal;
					} else if (el.className && el.className.toString) {
						classNameStr = el.className.toString();
					}
					
					if (classNameStr) {
						const classes = classNameStr.split(' ').filter(c => c).slice(0, 2).join('.');
						if (classes) {
							uniqueSelector = tagName + '.' + classes;
						}
					}
				}
				

				if (el.getAttribute('data-qa')) {
					uniqueSelector = '[data-qa="' + el.getAttribute('data-qa') + '"]';
				} else if (el.getAttribute('data-testid')) {
					uniqueSelector = '[data-testid="' + el.getAttribute('data-testid') + '"]';
				} else if (el.getAttribute('aria-label')) {
					uniqueSelector = tagName + '[aria-label="' + el.getAttribute('aria-label') + '"]';
				} else if (el.getAttribute('name')) {
					uniqueSelector = tagName + '[name="' + el.getAttribute('name') + '"]';
				} else if (el.getAttribute('type')) {
					uniqueSelector = tagName + '[type="' + el.getAttribute('type') + '"]';
				}
				

				
				const attributes = {};
				Array.from(el.attributes).forEach(attr => {
					if (attr.name.startsWith('data-') || attr.name === 'id' || attr.name === 'class') {
						attributes[attr.name] = attr.value;
					}
				});
				
				elements.push({
					type: tagName,
					selector: uniqueSelector,
					text: text.substring(0, 200),
					attributes: attributes,
					isVisible: isVisible,
					isClickable: el.disabled !== true && 
						(el.onclick !== null || el.tagName === 'A' || el.tagName === 'BUTTON' ||
						 el.getAttribute('role') === 'button' || el.style.cursor === 'pointer'),
					position: {
						x: Math.round(rect.left + rect.width / 2),
						y: Math.round(rect.top + rect.height / 2)
					}
				});
			});
		});
		

		const seen = new Set();
		const prioritized = [];
		const others = [];
		
		elements.forEach(el => {
			if (seen.has(el.selector)) return;
			seen.add(el.selector);
			

			if (el.selector.includes('data-qa') || el.selector.includes('data-testid') || el.selector.startsWith('#')) {
				prioritized.push(el);
			} else {
				others.push(el);
			}
		});
		

		return prioritized.concat(others).slice(0, 100);
	}
	`

	result, err := currentPage.Evaluate(jsCode)
	if err != nil {
		return nil, fmt.Errorf("failed to extract elements: %w", err)
	}

	elementsData, ok := result.([]interface{})
	if !ok {
		return []entities.PageElement{}, nil
	}

	elements := make([]entities.PageElement, 0, len(elementsData))
	for _, elData := range elementsData {
		elMap, ok := elData.(map[string]interface{})
		if !ok {
			continue
		}

		element := entities.PageElement{
			Type:        getString(elMap, "type"),
			Selector:    getString(elMap, "selector"),
			Text:        getString(elMap, "text"),
			Attributes:  make(map[string]string),
			IsVisible:   getBool(elMap, "isVisible"),
			IsClickable: getBool(elMap, "isClickable"),
		}

		if pos, ok := elMap["position"].(map[string]interface{}); ok {
			element.Position.X = getInt(pos, "x")
			element.Position.Y = getInt(pos, "y")
		}

		if attrs, ok := elMap["attributes"].(map[string]interface{}); ok {
			for k, v := range attrs {
				if str, ok := v.(string); ok {
					element.Attributes[k] = str
				}
			}
		}

		elements = append(elements, element)
	}

	return elements, nil
}

// GetTextContent - extracts visible text content from the page
func (b *browserController) GetTextContent(ctx context.Context) (string, error) {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	jsCode := `
	() => {
		const walker = document.createTreeWalker(
			document.body,
			NodeFilter.SHOW_TEXT,
			{
				acceptNode: function(node) {
					const parent = node.parentElement;
					if (!parent) return NodeFilter.FILTER_REJECT;
					const style = window.getComputedStyle(parent);
					if (style.display === 'none' || style.visibility === 'hidden') {
						return NodeFilter.FILTER_REJECT;
					}
					return NodeFilter.FILTER_ACCEPT;
				}
			}
		);
		
		const texts = [];
		let node;
		while (node = walker.nextNode()) {
			const text = node.textContent.trim();
			if (text.length > 0) {
				texts.push(text);
			}
		}
		
		return texts.join(' ').substring(0, 3000);
	}
	`

	result, err := currentPage.Evaluate(jsCode)
	if err != nil {
		return "", err
	}

	if text, ok := result.(string); ok {
		return text, nil
	}

	return "", nil
}

// Screenshot - takes a screenshot of the current page
func (b *browserController) Screenshot(ctx context.Context, path string) error {
	b.pagesMutex.Lock()
	currentPage := b.page
	b.pagesMutex.Unlock()

	_, err := currentPage.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(path),
	})
	return err
}

// SaveState - saves browser state to persistent storage
func (b *browserController) SaveState() error {
	if b.context == nil || b.storagePath == "" {
		return nil
	}

	_, err := b.context.StorageState(b.storagePath)
	if err != nil {

		errStr := err.Error()
		if strings.Contains(errStr, "closed") || strings.Contains(errStr, "target closed") {
			return nil
		}
		return fmt.Errorf("failed to save browser state: %w", err)
	}

	return nil
}

// OpenNewTab - opens a new browser tab and optionally navigates to URL
func (b *browserController) OpenNewTab(ctx context.Context, url string) error {
	b.pagesMutex.Lock()
	defer b.pagesMutex.Unlock()

	newPage, err := b.context.NewPage()
	if err != nil {
		return fmt.Errorf("failed to create new page: %w", err)
	}

	b.pages = append(b.pages, newPage)

	b.page = newPage

	if url != "" {
		_, err = newPage.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})
		if err != nil {
			return fmt.Errorf("failed to navigate to %s: %w", url, err)
		}
	} else {

		newPage.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateDomcontentloaded,
			Timeout: playwright.Float(5000),
		})
	}

	time.Sleep(200 * time.Millisecond)

	newPage.OnDialog(func(dialog playwright.Dialog) {

		dialog.Accept()
	})

	newPage.OnClose(func(closedPage playwright.Page) {
		b.pagesMutex.Lock()
		defer b.pagesMutex.Unlock()

		for i, p := range b.pages {
			if p == closedPage {
				b.pages = append(b.pages[:i], b.pages[i+1:]...)
				break
			}
		}

		if b.page == closedPage && len(b.pages) > 0 {
			b.page = b.pages[0]
		}
	})

	return nil
}

// SwitchToTab - switches to a tab by index
func (b *browserController) SwitchToTab(index int) error {
	b.pagesMutex.Lock()
	defer b.pagesMutex.Unlock()

	if index < 0 || index >= len(b.pages) {
		return fmt.Errorf("invalid tab index: %d (available tabs: %d)", index, len(b.pages))
	}

	b.page = b.pages[index]

	b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(3000),
	})

	time.Sleep(200 * time.Millisecond)

	return nil
}

// GetTabsCount - returns the number of open tabs
func (b *browserController) GetTabsCount() int {
	b.pagesMutex.Lock()
	defer b.pagesMutex.Unlock()
	return len(b.pages)
}

// GetCurrentTabIndex - returns the index of the current active tab
func (b *browserController) GetCurrentTabIndex() int {
	b.pagesMutex.Lock()
	defer b.pagesMutex.Unlock()

	for i, p := range b.pages {
		if p == b.page {
			return i
		}
	}
	return 0
}

// Close - closes the browser and saves state
func (b *browserController) Close() error {
	var closeErr error

	if err := b.SaveState(); err != nil {

		errStr := err.Error()
		if !strings.Contains(errStr, "closed") && !strings.Contains(errStr, "target closed") {
			closeErr = err
		}
	}

	if b.context != nil {
		if err := b.context.Close(); err != nil {
			errStr := err.Error()

			if !strings.Contains(errStr, "closed") && !strings.Contains(errStr, "target closed") {
				if closeErr != nil {
					closeErr = fmt.Errorf("%v; failed to close context: %w", closeErr, err)
				} else {
					closeErr = fmt.Errorf("failed to close context: %w", err)
				}
			}
		}
		b.context = nil
	}

	if b.browser != nil {
		if err := b.browser.Close(); err != nil {
			errStr := err.Error()

			if !strings.Contains(errStr, "closed") && !strings.Contains(errStr, "target closed") {
				if closeErr != nil {
					closeErr = fmt.Errorf("%v; failed to close browser: %w", closeErr, err)
				} else {
					closeErr = fmt.Errorf("failed to close browser: %w", err)
				}
			}
		}
		b.browser = nil
	}

	return closeErr
}

// getString - extracts string value from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getBool - extracts boolean value from map
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getInt - extracts integer value from map
func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}

// truncateString - truncates string to maximum length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
