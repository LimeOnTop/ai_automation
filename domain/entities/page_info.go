package entities

// PageElement represents a single element on the page
type PageElement struct {
	TagName      string            `json:"tag_name"`
	Text         string            `json:"text"`
	Placeholder  string            `json:"placeholder,omitempty"`
	Value        string            `json:"value,omitempty"`
	Attributes   map[string]string `json:"attributes"`
	Selector     string            `json:"selector"`
	AllSelectors []string          `json:"all_selectors,omitempty"`
	IsVisible    bool              `json:"is_visible"`
	IsClickable  bool              `json:"is_clickable"`
	XPath        string            `json:"xpath,omitempty"`
}

// PageInfo represents structured information about the current page
type PageInfo struct {
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Elements    []PageElement  `json:"elements"`
	TextContent string         `json:"text_content"`
	Links       []LinkInfo     `json:"links"`
	Forms       []FormInfo     `json:"forms"`
	Buttons     []PageElement  `json:"buttons"`
}

// LinkInfo represents a link on the page
type LinkInfo struct {
	Text     string `json:"text"`
	URL      string `json:"url"`
	Href     string `json:"href"`
	Selector string `json:"selector,omitempty"`
}

// FormInfo represents a form on the page
type FormInfo struct {
	Action     string            `json:"action"`
	Method     string            `json:"method"`
	Inputs     []InputInfo      `json:"inputs"`
	SubmitText string            `json:"submit_text,omitempty"`
}

// InputInfo represents an input field
type InputInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Placeholder string `json:"placeholder,omitempty"`
	Label       string `json:"label,omitempty"`
	Value       string `json:"value,omitempty"`
}

