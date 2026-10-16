package rules

import (
	"context"
	"testing"

	"github.com/max25/smart-cart-service/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDeliveryRule(t *testing.T) {
	tests := []struct {
		name          string
		final         domain.Money
		threshold     domain.Money
		wantAchieved  bool
		wantRemaining domain.Money
		wantPercent   int
	}{
		{
			name:          "сумма ниже порога",
			final:         money(115000),
			threshold:     FreeShippingThreshold,
			wantAchieved:  false,
			wantRemaining: money(35000),
			wantPercent:   76,
		},
		{
			name:          "сумма ровно равна порогу",
			final:         money(150000),
			threshold:     FreeShippingThreshold,
			wantAchieved:  true,
			wantRemaining: money(0),
			wantPercent:   100,
		},
		{
			name:          "сумма выше порога",
			final:         money(200000),
			threshold:     FreeShippingThreshold,
			wantAchieved:  true,
			wantRemaining: money(0),
			wantPercent:   100,
		},
		{
			name:          "пустая корзина",
			final:         money(0),
			threshold:     FreeShippingThreshold,
			wantAchieved:  false,
			wantRemaining: money(150000),
			wantPercent:   0,
		},
		{
			name:          "кастомный порог",
			final:         money(50000),
			threshold:     money(100000),
			wantAchieved:  false,
			wantRemaining: money(50000),
			wantPercent:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc := &CalculatedCart{Final: tt.final}
			deps := &Deps{FreeShippingThreshold: tt.threshold}

			require.NoError(t, NewDeliveryRule().Apply(context.Background(), cc, deps))

			require.Equal(t, tt.threshold, cc.FreeShipping.Threshold)
			require.Equal(t, tt.final, cc.FreeShipping.Current)
			require.Equal(t, tt.wantAchieved, cc.FreeShipping.Achieved)
			require.Equal(t, tt.wantRemaining, cc.FreeShipping.Remaining)
			require.Equal(t, tt.wantPercent, cc.FreeShipping.Percent)
		})
	}
}
