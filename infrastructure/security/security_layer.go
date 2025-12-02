package security

import (
	"context"
	"strings"

	"ai_automation/domain/entities"
	"ai_automation/domain/interfaces"

	"github.com/sirupsen/logrus"
)

type SecurityLayer struct {
	logger *logrus.Logger
}

func NewSecurityLayer(logger *logrus.Logger) *SecurityLayer {
	return &SecurityLayer{
		logger: logger,
	}
}

func (s *SecurityLayer) RequiresApproval(ctx context.Context, action *entities.Action, pageInfo *entities.PageInfo) bool {
	if s.IsDestructiveAction(ctx, action) {
		return true
	}
	
	// Check for payment-related actions
	if s.isPaymentAction(ctx, action, pageInfo) {
		return true
	}
	
	// Check for deletion actions
	if s.isDeletionAction(ctx, action, pageInfo) {
		return true
	}
	
	// Check for form submissions that might be critical
	if s.isCriticalFormSubmission(ctx, action, pageInfo) {
		return true
	}
	
	return false
}

func (s *SecurityLayer) IsDestructiveAction(ctx context.Context, action *entities.Action) bool {
	// Check action type
	if action.Type == entities.ActionClick {
		// Check if clicking on delete, remove, or similar buttons
		lowerSelector := strings.ToLower(action.Selector)
		lowerDesc := strings.ToLower(action.Description)
		
		destructiveKeywords := []string{
			"delete", "remove", "удалить", "удаление",
			"cancel", "отменить", "отмена",
			"clear", "очистить",
			"reset", "сброс",
		}
		
		for _, keyword := range destructiveKeywords {
			if strings.Contains(lowerSelector, keyword) || strings.Contains(lowerDesc, keyword) {
				return true
			}
		}
	}
	
	return false
}

func (s *SecurityLayer) GetActionRiskLevel(ctx context.Context, action *entities.Action) string {
	if s.IsDestructiveAction(ctx, action) {
		return "high"
	}
	
	if action.Type == entities.ActionNavigate {
		// Navigation is generally low risk
		return "low"
	}
	
	if action.Type == entities.ActionTypeText {
		// Typing text could be medium risk if it's in forms
		return "medium"
	}
	
	if action.Type == entities.ActionClick {
		// Clicking could be medium to high risk depending on context
		return "medium"
	}
	
	return "low"
}

func (s *SecurityLayer) isPaymentAction(ctx context.Context, action *entities.Action, pageInfo *entities.PageInfo) bool {
	if pageInfo == nil {
		return false
	}
	
	// Check URL for payment-related keywords
	lowerURL := strings.ToLower(pageInfo.URL)
	paymentKeywords := []string{
		"payment", "pay", "checkout", "оплата", "платеж",
		"order", "заказ", "purchase", "покупка",
	}
	
	for _, keyword := range paymentKeywords {
		if strings.Contains(lowerURL, keyword) {
			// If we're on a payment page and clicking submit/confirm
			if action.Type == entities.ActionClick {
				lowerSelector := strings.ToLower(action.Selector)
				lowerDesc := strings.ToLower(action.Description)
				
				confirmKeywords := []string{
					"submit", "confirm", "pay", "оплатить", "подтвердить",
					"order", "заказать", "buy", "купить",
				}
				
				for _, confirmKeyword := range confirmKeywords {
					if strings.Contains(lowerSelector, confirmKeyword) || strings.Contains(lowerDesc, confirmKeyword) {
						return true
					}
				}
			}
		}
	}
	
	return false
}

func (s *SecurityLayer) isDeletionAction(ctx context.Context, action *entities.Action, pageInfo *entities.PageInfo) bool {
	if action.Type != entities.ActionClick {
		return false
	}
	
	lowerSelector := strings.ToLower(action.Selector)
	lowerDesc := strings.ToLower(action.Description)
	
	deletionKeywords := []string{
		"delete", "удалить", "remove", "удаление",
		"trash", "корзина", "clear", "очистить",
	}
	
	for _, keyword := range deletionKeywords {
		if strings.Contains(lowerSelector, keyword) || strings.Contains(lowerDesc, keyword) {
			return true
		}
	}
	
	return false
}

func (s *SecurityLayer) isCriticalFormSubmission(ctx context.Context, action *entities.Action, pageInfo *entities.PageInfo) bool {
	if pageInfo == nil {
		return false
	}
	
	if action.Type == entities.ActionClick {
		// Check if we're submitting a form
		lowerSelector := strings.ToLower(action.Selector)
		lowerDesc := strings.ToLower(action.Description)
		
		submitKeywords := []string{
			"submit", "send", "отправить", "подтвердить",
		}
		
		for _, keyword := range submitKeywords {
			if strings.Contains(lowerSelector, keyword) || strings.Contains(lowerDesc, keyword) {
				// If there are forms on the page, this might be critical
				if len(pageInfo.Forms) > 0 {
					return true
				}
			}
		}
	}
	
	return false
}

// Ensure SecurityLayer implements SecurityLayer interface
var _ interfaces.SecurityLayer = (*SecurityLayer)(nil)

