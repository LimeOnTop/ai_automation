package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"

	"github.com/sirupsen/logrus"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

type SeleniumController struct {
	wd          selenium.WebDriver
	service     *selenium.Service
	logger      *logrus.Logger
	userDataDir string
}

// findChromeDriver - finds ChromeDriver executable path
func findChromeDriver() (string, error) {
	if path := os.Getenv("BROWSER_DRIVER_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	commonPaths := []string{
		"/usr/local/bin/chromedriver",
		"/usr/bin/chromedriver",
		"/opt/homebrew/bin/chromedriver",
		filepath.Join(os.Getenv("HOME"), "bin", "chromedriver"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	if path, err := exec.LookPath("chromedriver"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("chromedriver not found. Please install it or set BROWSER_DRIVER_PATH environment variable")
}

// findChromeBinary - finds Chrome/Chromium browser executable path
func findChromeBinary() string {
	if path := os.Getenv("CHROME_BINARY_PATH"); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	chromePaths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}

	for _, path := range chromePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	if path, err := exec.LookPath("google-chrome"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium"); err == nil {
		return path
	}
	if path, err := exec.LookPath("chromium-browser"); err == nil {
		return path
	}

	return ""
}

// getOrCreateUserDataDir - gets or creates user data directory for persistent sessions
func getOrCreateUserDataDir() (string, error) {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		return "", fmt.Errorf("HOME environment variable is not set")
	}

	userDataDir := filepath.Join(homeDir, ".ai_automation", "chrome_profile")
	if err := os.MkdirAll(userDataDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create user data directory: %w", err)
	}

	return userDataDir, nil
}

// NewSeleniumController - creates new Selenium browser controller instance
func NewSeleniumController(logger *logrus.Logger) (*SeleniumController, error) {
	driverPath, err := findChromeDriver()
	if err != nil {
		return nil, fmt.Errorf("failed to find chromedriver: %w", err)
	}

	logger.Infof("Using ChromeDriver at: %s", driverPath)

	chromeBinary := findChromeBinary()
	if chromeBinary != "" {
		logger.Infof("Using Chrome binary at: %s", chromeBinary)
	}

	userDataDir, err := getOrCreateUserDataDir()
	if err != nil {
		return nil, fmt.Errorf("failed to setup user data directory: %w", err)
	}
	logger.Infof("Using user data directory: %s (sessions will be preserved)", userDataDir)

	opts := []selenium.ServiceOption{}
	service, err := selenium.NewChromeDriverService(driverPath, 9515, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to start chromedriver: %w", err)
	}

	caps := selenium.Capabilities{
		"browserName": "chrome",
	}

	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			fmt.Sprintf("--user-data-dir=%s", userDataDir),
		},
	}

	if chromeBinary != "" {
		chromeCaps.Path = chromeBinary
	}

	caps.AddChrome(chromeCaps)

	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", 9515))
	if err != nil {
		service.Stop()
		if strings.Contains(err.Error(), "cannot find Chrome binary") {
			return nil, fmt.Errorf("failed to create webdriver: Chrome browser not found. Please install Google Chrome or set CHROME_BINARY_PATH environment variable. Error: %w", err)
		}
		return nil, fmt.Errorf("failed to create webdriver: %w", err)
	}

	return &SeleniumController{
		wd:          wd,
		service:     service,
		logger:      logger,
		userDataDir: userDataDir,
	}, nil
}

// Navigate - navigates browser to specified URL
func (s *SeleniumController) Navigate(ctx context.Context, url string) error {
	s.logger.Infof("Navigating to: %s", url)
	return s.wd.Get(url)
}

// Click - clicks on element identified by selector
func (s *SeleniumController) Click(ctx context.Context, selector string) error {
	s.logger.Infof("Clicking on: %s", selector)

	element, err := s.findElement(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}

	// Scroll element into view using JavaScript for better reliability
	script := `
	(function() {
		var element = arguments[0];
		element.scrollIntoView({ behavior: 'smooth', block: 'center' });
		return true;
	})();
	`
	_, err = s.wd.ExecuteScript(script, []interface{}{element})
	if err != nil {
		s.logger.Warnf("Failed to scroll to element: %v", err)
		// Try alternative method
		if err := element.MoveTo(0, 0); err != nil {
			s.logger.Warnf("Failed to move to element: %v", err)
		}
	}

	time.Sleep(300 * time.Millisecond)
	return element.Click()
}

// TypeText - types text into input field identified by selector
func (s *SeleniumController) TypeText(ctx context.Context, selector string, text string) error {
	s.logger.Infof("Typing text into: %s", selector)

	element, err := s.findElement(selector)
	if err != nil {
		return fmt.Errorf("element not found: %w", err)
	}

	if err := element.Clear(); err != nil {
		s.logger.Warnf("Failed to clear element: %v", err)
	}

	for _, char := range text {
		if err := element.SendKeys(string(char)); err != nil {
			return fmt.Errorf("failed to type character: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

// ExtractPageInfo - extracts structured information from current page
func (s *SeleniumController) ExtractPageInfo(ctx context.Context) (*entities.PageInfo, error) {
	s.logger.Debug("Extracting page info")

	url, err := s.GetCurrentURL(ctx)
	if err != nil {
		return nil, err
	}

	title, err := s.GetPageTitle(ctx)
	if err != nil {
		return nil, err
	}

	elements, err := s.extractElements(ctx)
	if err != nil {
		s.logger.Warnf("Failed to extract elements: %v", err)
		elements = []entities.PageElement{}
	}

	links, err := s.extractLinks(ctx)
	if err != nil {
		s.logger.Warnf("Failed to extract links: %v", err)
		links = []entities.LinkInfo{}
	}

	forms, err := s.extractForms(ctx)
	if err != nil {
		s.logger.Warnf("Failed to extract forms: %v", err)
		forms = []entities.FormInfo{}
	}

	buttons, err := s.extractButtons(ctx)
	if err != nil {
		s.logger.Warnf("Failed to extract buttons: %v", err)
		buttons = []entities.PageElement{}
	}

	textContent, err := s.getVisibleText(ctx)
	if err != nil {
		textContent = ""
	}

	return &entities.PageInfo{
		URL:         url,
		Title:       title,
		Description: s.generateDescription(elements, links, forms),
		Elements:    elements,
		TextContent: textContent,
		Links:       links,
		Forms:       forms,
		Buttons:     buttons,
	}, nil
}

// Wait - waits for specified timeout
func (s *SeleniumController) Wait(ctx context.Context, condition string, timeout int) error {
	if timeout == 0 {
		timeout = 5
	}

	time.Sleep(time.Duration(timeout) * time.Second)
	return nil
}

// Scroll - scrolls page in specified direction
func (s *SeleniumController) Scroll(ctx context.Context, direction string, amount int) error {
	if amount == 0 {
		amount = 500
	}

	script := ""
	if direction == "down" || direction == "" {
		script = fmt.Sprintf("window.scrollBy(0, %d);", amount)
	} else if direction == "up" {
		script = fmt.Sprintf("window.scrollBy(0, -%d);", amount)
	}

	_, err := s.wd.ExecuteScript(script, nil)
	return err
}

// GetCurrentURL - returns current page URL
func (s *SeleniumController) GetCurrentURL(ctx context.Context) (string, error) {
	return s.wd.CurrentURL()
}

// GetPageTitle - returns current page title
func (s *SeleniumController) GetPageTitle(ctx context.Context) (string, error) {
	return s.wd.Title()
}

// TakeScreenshot - takes screenshot of current page
func (s *SeleniumController) TakeScreenshot(ctx context.Context) ([]byte, error) {
	return s.wd.Screenshot()
}

// Close - closes browser and stops ChromeDriver service
func (s *SeleniumController) Close() error {
	if s.wd != nil {
		s.wd.Quit()
	}
	if s.service != nil {
		s.service.Stop()
	}
	return nil
}

// IsElementVisible - checks if element is visible on page
func (s *SeleniumController) IsElementVisible(ctx context.Context, selector string) (bool, error) {
	element, err := s.findElement(selector)
	if err != nil {
		return false, nil
	}

	return element.IsDisplayed()
}

// FindElementsByText - finds elements containing specified text
func (s *SeleniumController) FindElementsByText(ctx context.Context, text string) ([]entities.PageElement, error) {
	xpath := fmt.Sprintf("//*[contains(text(), '%s')]", text)
	elements, err := s.wd.FindElements(selenium.ByXPATH, xpath)
	if err != nil {
		return nil, err
	}

	result := make([]entities.PageElement, 0, len(elements))
	for _, elem := range elements {
		tagName, _ := elem.TagName()
		elemText, _ := elem.Text()
		isVisible, _ := elem.IsDisplayed()

		result = append(result, entities.PageElement{
			TagName:   tagName,
			Text:      elemText,
			IsVisible: isVisible,
		})
	}

	return result, nil
}

// findElement - finds element using various selector strategies
func (s *SeleniumController) findElement(selector string) (selenium.WebElement, error) {
	strategies := []struct {
		by    string
		value string
	}{
		{selenium.ByCSSSelector, selector},
		{selenium.ByXPATH, selector},
		{selenium.ByID, selector},
		{selenium.ByLinkText, selector},
		{selenium.ByPartialLinkText, selector},
	}

	if !strings.Contains(selector, "/") && !strings.Contains(selector, "[") && !strings.Contains(selector, "#") && !strings.Contains(selector, ".") {
		textXPath := fmt.Sprintf("//*[contains(text(), '%s')]", selector)
		element, err := s.wd.FindElement(selenium.ByXPATH, textXPath)
		if err == nil {
			return element, nil
		}

		buttonXPath := fmt.Sprintf("//button[contains(text(), '%s')]", selector)
		element, err = s.wd.FindElement(selenium.ByXPATH, buttonXPath)
		if err == nil {
			return element, nil
		}

		linkXPath := fmt.Sprintf("//a[contains(text(), '%s')]", selector)
		element, err = s.wd.FindElement(selenium.ByXPATH, linkXPath)
		if err == nil {
			return element, nil
		}
	}

	for _, strategy := range strategies {
		element, err := s.wd.FindElement(strategy.by, strategy.value)
		if err == nil {
			return element, nil
		}
	}

	return nil, fmt.Errorf("element not found with selector: %s", selector)
}

// extractElements - extracts interactive elements from page using JavaScript
func (s *SeleniumController) extractElements(ctx context.Context) ([]entities.PageElement, error) {
	script := `
	(function() {
		const elements = [];
		const interactiveSelectors = [
			'button', 'a', 'input', 'select', 'textarea',
			'[role="button"]', '[role="link"]', '[role="listitem"]',
			'[onclick]', '[data-testid]', '[data-qa]',
			'[class*="button"]', '[class*="btn"]', '[class*="link"]',
			'[class*="clickable"]', '[class*="item"]', '[class*="row"]',
			'[class*="snippet"]', '[class*="list"]',
			'tr[data-key]', 'tr[onclick]',
			'li[onclick]', 'div[onclick]', 'span[onclick]'
		];
		const interactiveElements = [];
		
		// First, collect all interactive elements (including those not in viewport)
		interactiveSelectors.forEach(selector => {
			try {
				document.querySelectorAll(selector).forEach(el => {
					const style = window.getComputedStyle(el);
					const isHidden = style.visibility === 'hidden' || style.display === 'none';
					
					if (isHidden) return;
					
					// Check if element exists in DOM (even if not in viewport)
					const rect = el.getBoundingClientRect();
					const hasSize = rect.width > 0 && rect.height > 0;
					
					// Skip elements with zero size (truly invisible)
					if (!hasSize && rect.width === 0 && rect.height === 0) return;
					
					// Element is "visible" if it's not hidden by CSS (even if outside viewport)
					// We include all elements with size, even if outside viewport
					const isVisible = hasSize;
					
					const attrs = {};
					for (let attr of el.attributes) {
						attrs[attr.name] = attr.value;
					}
					
					// Generate multiple selector options
					let selectors = [];
					if (el.id) selectors.push('#' + el.id);
					if (el.className && el.className.trim()) {
						el.className.trim().split(/\s+/).forEach(cls => {
							if (cls && cls.length < 50) selectors.push('.' + cls);
						});
					}
					if (el.getAttribute('data-testid')) {
						selectors.push('[data-testid="' + el.getAttribute('data-testid') + '"]');
					}
					if (el.getAttribute('data-qa')) {
						selectors.push('[data-qa="' + el.getAttribute('data-qa') + '"]');
					}
					if (el.getAttribute('name')) {
						selectors.push('[name="' + el.getAttribute('name') + '"]');
					}
					if (el.getAttribute('data-key')) {
						selectors.push('[data-key="' + el.getAttribute('data-key') + '"]');
					}
					
					let primarySelector = el.tagName.toLowerCase();
					if (selectors.length > 0) {
						primarySelector = selectors[0];
					} else if (el.className && el.className.trim()) {
						const firstClass = el.className.trim().split(/\s+/)[0];
						if (firstClass) primarySelector += '.' + firstClass;
					}
					
					const text = el.textContent ? el.textContent.trim().substring(0, 200) : '';
					const placeholder = el.placeholder || '';
					const value = el.value || '';
					
					// For list items and table rows, include more context
					let displayText = text;
					if ((el.tagName === 'TR' || el.tagName === 'LI' || el.getAttribute('role') === 'listitem') && text.length > 20) {
						displayText = text.substring(0, 150);
					}
					
					interactiveElements.push({
						tag_name: el.tagName.toLowerCase(),
						text: displayText,
						placeholder: placeholder,
						value: value,
						attributes: attrs,
						selector: primarySelector,
						all_selectors: selectors,
						is_visible: isVisible,
						is_clickable: true
					});
				});
			} catch(e) {}
		});
		
		// Remove duplicates and increase limit to get all elements
		const seen = new Set();
		const unique = [];
		interactiveElements.forEach(el => {
			const key = el.selector + '|' + el.text.substring(0, 50);
			if (!seen.has(key) && unique.length < 100) {
				seen.add(key);
				unique.push(el);
			}
		});
		
		return unique;
	})();
	`

	var result []entities.PageElement
	rawResult, err := s.wd.ExecuteScript(script, nil)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(rawResult)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// extractLinks - extracts links from page using JavaScript
func (s *SeleniumController) extractLinks(ctx context.Context) ([]entities.LinkInfo, error) {
	script := `
	(function() {
		const links = [];
		const allLinks = document.querySelectorAll('a[href]');
		const seen = new Set();
		
		for (let i = 0; i < allLinks.length && links.length < 100; i++) {
			const link = allLinks[i];
			const style = window.getComputedStyle(link);
			const isHidden = style.visibility === 'hidden' || style.display === 'none';
			
			if (isHidden) continue;
			
			// Include links even if outside viewport
			const rect = link.getBoundingClientRect();
			const hasSize = rect.width > 0 && rect.height > 0;
			
			if (!hasSize && rect.width === 0 && rect.height === 0) continue;
			
			const text = link.textContent ? link.textContent.trim().substring(0, 150) : '';
			const href = link.getAttribute('href') || '';
			const key = text + '|' + href;
			
			if (seen.has(key)) continue;
			seen.add(key);
			
			// Generate selector
			let selector = 'a';
			if (link.id) {
				selector = 'a#' + link.id;
			} else if (link.className && link.className.trim()) {
				const classes = link.className.trim().split(/\s+/).filter(c => c && !c.includes(' '));
				if (classes.length > 0) {
					selector = 'a.' + classes[0];
				}
			}
			if (link.getAttribute('data-testid')) {
				selector = 'a[data-testid="' + link.getAttribute('data-testid') + '"]';
			}
			if (link.getAttribute('data-qa')) {
				selector = 'a[data-qa="' + link.getAttribute('data-qa') + '"]';
			}
			
			links.push({
				text: text,
				url: link.href,
				href: href,
				selector: selector
			});
		}
		
		return links;
	})();
	`

	var result []entities.LinkInfo
	rawResult, err := s.wd.ExecuteScript(script, nil)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(rawResult)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// extractForms - extracts forms from page using JavaScript
func (s *SeleniumController) extractForms(ctx context.Context) ([]entities.FormInfo, error) {
	script := `
	(function() {
		const forms = [];
		const allForms = document.querySelectorAll('form');
		
		for (let form of allForms) {
			const inputs = [];
			const formInputs = form.querySelectorAll('input, textarea, select');
			
			for (let input of formInputs) {
				inputs.push({
					type: input.type || input.tagName.toLowerCase(),
					name: input.name || '',
					placeholder: input.placeholder || '',
					value: input.value || ''
				});
			}
			
			const submitBtn = form.querySelector('button[type="submit"], input[type="submit"]');
			
			forms.push({
				action: form.action || '',
				method: form.method || 'get',
				inputs: inputs,
				submit_text: submitBtn ? (submitBtn.textContent || submitBtn.value || '') : ''
			});
		}
		
		return forms;
	})();
	`

	var result []entities.FormInfo
	rawResult, err := s.wd.ExecuteScript(script, nil)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(rawResult)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// extractButtons - extracts buttons from page using JavaScript
func (s *SeleniumController) extractButtons(ctx context.Context) ([]entities.PageElement, error) {
	script := `
	(function() {
		const buttons = [];
		const selectors = [
			'button',
			'input[type="button"]',
			'input[type="submit"]',
			'a[role="button"]',
			'[role="button"]',
			'.button',
			'[class*="button"]',
			'[class*="btn"]',
			'[data-testid*="button"]',
			'[data-qa*="button"]'
		];
		
		const seen = new Set();
		
		selectors.forEach(selector => {
			try {
				document.querySelectorAll(selector).forEach(btn => {
					const style = window.getComputedStyle(btn);
					const isHidden = style.visibility === 'hidden' || style.display === 'none';
					
					if (isHidden) return;
					
					// Include buttons even if outside viewport
					const rect = btn.getBoundingClientRect();
					const hasSize = rect.width > 0 && rect.height > 0;
					
					// Skip buttons with zero size (truly invisible)
					if (!hasSize && rect.width === 0 && rect.height === 0) return;
					
					const text = btn.textContent ? btn.textContent.trim().substring(0, 150) : (btn.value || '');
					const key = btn.tagName + '|' + text + '|' + (btn.id || '');
					
					if (seen.has(key) || buttons.length >= 80) return;
					seen.add(key);
					
					// Generate selector
					let selectorStr = btn.tagName.toLowerCase();
					if (btn.id) {
						selectorStr = '#' + btn.id;
					} else if (btn.className && btn.className.trim()) {
						const classes = btn.className.trim().split(/\s+/).filter(c => c && !c.includes(' '));
						if (classes.length > 0) {
							selectorStr += '.' + classes[0];
						}
					}
					if (btn.getAttribute('data-testid')) {
						selectorStr = '[data-testid="' + btn.getAttribute('data-testid') + '"]';
					}
					if (btn.getAttribute('data-qa')) {
						selectorStr = '[data-qa="' + btn.getAttribute('data-qa') + '"]';
					}
					
					buttons.push({
						tag_name: btn.tagName.toLowerCase(),
						text: text,
						attributes: {},
						selector: selectorStr,
						is_visible: isVisible,
						is_clickable: true
					});
				});
			} catch(e) {}
		});
		
		return buttons;
	})();
	`

	var result []entities.PageElement
	rawResult, err := s.wd.ExecuteScript(script, nil)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(rawResult)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// getVisibleText - extracts visible text content from page
func (s *SeleniumController) getVisibleText(ctx context.Context) (string, error) {
	script := `
	(function() {
		// Extract text from clickable elements first (list items, table rows, etc.)
		const clickableTexts = [];
		const clickableSelectors = [
			'[role="listitem"]',
			'[role="option"]',
			'tr[data-key]',
			'tr[onclick]',
			'li[onclick]',
			'[class*="item"]',
			'[class*="row"]',
			'[class*="snippet"]',
			'li',
			'tr'
		];
		
		clickableSelectors.forEach(sel => {
			try {
				document.querySelectorAll(sel).forEach(el => {
					const rect = el.getBoundingClientRect();
					const isVisible = rect.width > 0 && rect.height > 0 &&
						window.getComputedStyle(el).visibility !== 'hidden' &&
						window.getComputedStyle(el).display !== 'none';
					
					if (!isVisible || clickableTexts.length >= 50) return;
					
					const text = el.textContent ? el.textContent.trim() : '';
					if (text.length > 10 && text.length < 500) {
						clickableTexts.push(text.substring(0, 200));
					}
				});
			} catch(e) {}
		});
		
		// Also get general visible text
		const walker = document.createTreeWalker(
			document.body,
			NodeFilter.SHOW_TEXT,
			null,
			false
		);
		
		let text = clickableTexts.join(' | ') + ' | ';
		let node;
		let charCount = 0;
		while ((node = walker.nextNode()) && charCount < 1500) {
			const parent = node.parentElement;
			if (parent && window.getComputedStyle(parent).display !== 'none') {
				const nodeText = node.textContent ? node.textContent.trim() : '';
				if (nodeText.length > 3) {
					text += nodeText + ' ';
					charCount += nodeText.length;
				}
			}
		}
		
		return text.trim().substring(0, 2000);
	})();
	`

	result, err := s.wd.ExecuteScript(script, nil)
	if err != nil {
		return "", err
	}

	if text, ok := result.(string); ok {
		return text, nil
	}

	return "", nil
}

// generateDescription - generates page description from extracted elements
func (s *SeleniumController) generateDescription(elements []entities.PageElement, links []entities.LinkInfo, forms []entities.FormInfo) string {
	parts := []string{}

	if len(links) > 0 {
		parts = append(parts, fmt.Sprintf("%d links", len(links)))
	}
	if len(forms) > 0 {
		parts = append(parts, fmt.Sprintf("%d forms", len(forms)))
	}
	if len(elements) > 0 {
		parts = append(parts, fmt.Sprintf("%d interactive elements", len(elements)))
	}

	return strings.Join(parts, ", ")
}

// Ensure SeleniumController implements BrowserController interface
var _ interfaces.BrowserController = (*SeleniumController)(nil)
