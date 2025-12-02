package entities

// ActionType represents the type of action agent can perform
type ActionType string

const (
	ActionNavigate   ActionType = "navigate"
	ActionClick      ActionType = "click"
	ActionTypeText   ActionType = "type"
	ActionExtract    ActionType = "extract"
	ActionWait       ActionType = "wait"
	ActionScroll     ActionType = "scroll"
	ActionScreenshot ActionType = "screenshot"
)

// Action represents a single action the agent wants to perform
type Action struct {
	Type             ActionType `json:"type"`
	Selector         string     `json:"selector,omitempty"`
	Text             string     `json:"text,omitempty"`
	URL              string     `json:"url,omitempty"`
	Description      string     `json:"description"`
	RequiresApproval bool       `json:"requires_approval,omitempty"`
}

// ActionResult represents the result of an action
type ActionResult struct {
	Success  bool      `json:"success"`
	Message  string    `json:"message"`
	Data     string    `json:"data,omitempty"`
	Error    string    `json:"error,omitempty"`
	PageInfo *PageInfo `json:"page_info,omitempty"`
}
