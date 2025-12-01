package entities

// PageInfo содержит информацию о текущей странице
type PageInfo struct {
	URL         string        `json:"url"`
	Title       string        `json:"title"`
	Description string        `json:"description"` // Мета-описание или первые 500 символов текста
	Elements    []PageElement `json:"elements"`    // Ключевые интерактивные элементы
	TextContent string        `json:"text_content"` // Релевантный текстовый контент (урезанный)
}
