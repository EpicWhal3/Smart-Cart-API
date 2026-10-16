package rules

import (
	"context"
)

// DeliveryRule рассчитывает порог бесплатной доставки по итоговой сумме корзины.
type DeliveryRule struct{}

// NewDeliveryRule создаёт правило бесплатной доставки.
func NewDeliveryRule() DeliveryRule { return DeliveryRule{} }

// Name возвращает имя правила.
func (DeliveryRule) Name() string { return "delivery" }

// Apply применяет правило к корзине.
func (DeliveryRule) Apply(_ context.Context, cc *CalculatedCart, deps *Deps) error {
	threshold := FreeShippingThreshold
	if deps != nil && deps.FreeShippingThreshold > 0 {
		threshold = deps.FreeShippingThreshold
	}

	current := max(cc.Final, 0)

	remaining := max(threshold-current, 0)

	percent := 0
	if threshold > 0 {
		percent = min(int(current*100/threshold), 100)
	}

	cc.FreeShipping = FreeShippingInfo{
		Threshold: threshold,
		Current:   current,
		Remaining: remaining,
		Achieved:  current >= threshold && threshold > 0,
		Percent:   percent,
	}

	return nil
}
