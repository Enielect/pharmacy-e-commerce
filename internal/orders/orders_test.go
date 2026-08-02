package orders

import (
	"testing"
)

func TestCalculateTotal(t *testing.T) {
	tests := []struct {
		name  string
		items []LineItem
		want  int32
	}{
		{
			name:  "empty items",
			items: []LineItem{},
			want:  0,
		},
		{
			name: "single item",
			items: []LineItem{
				{VariantID: 1, PriceCents: 1000, Quantity: 2},
			},
			want: 2000,
		},
		{
			name: "multiple items",
			items: []LineItem{
				{VariantID: 1, PriceCents: 1500, Quantity: 3},
				{VariantID: 2, PriceCents: 2000, Quantity: 1},
				{VariantID: 3, PriceCents: 750, Quantity: 4},
			},
			want: 1500*3 + 2000*1 + 750*4,
		},
		{
			name: "zero quantity item",
			items: []LineItem{
				{VariantID: 1, PriceCents: 500, Quantity: 0},
				{VariantID: 2, PriceCents: 1000, Quantity: 1},
			},
			want: 1000,
		},
		{
			name: "large quantities",
			items: []LineItem{
				{VariantID: 1, PriceCents: 250, Quantity: 100},
			},
			want: 25000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTotal(tt.items)
			if got != tt.want {
				t.Errorf("CalculateTotal() = %d, want %d", got, tt.want)
			}
		})
	}
}
