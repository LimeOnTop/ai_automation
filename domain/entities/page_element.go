package entities

// PageElement представляет элемент на веб-странице
type PageElement struct {
	Type        string            `json:"type"`        // button, link, input, text, etc.
	Selector    string            `json:"selector"`    // CSS селектор
	Text        string            `json:"text"`        // Видимый текст
	Attributes  map[string]string `json:"attributes"`  // HTML атрибуты
	IsVisible   bool              `json:"is_visible"`  // Видим ли элемент
	IsClickable bool              `json:"is_clickable"` // Кликабелен ли элемент
	Position    Position          `json:"position"`    // Позиция на странице
}

// Position представляет позицию элемента
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

