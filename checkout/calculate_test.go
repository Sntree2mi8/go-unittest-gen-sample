package checkout

import (
	"context"
	"reflect"
	"testing"
)

type couponStoreFunc func(context.Context, string) (Coupon, error)

func (f couponStoreFunc) Find(ctx context.Context, code string) (Coupon, error) {
	return f(ctx, code)
}

func TestCalculate_AppliesDiscountsAndFreeShipping(t *testing.T) {
	t.Parallel()

	type contextKey string
	const requestIDKey contextKey = "request-id"
	ctx := context.WithValue(context.Background(), requestIDKey, "req-123")

	store := couponStoreFunc(func(gotCtx context.Context, gotCode string) (Coupon, error) {
		if gotCode != "SAVE10" {
			t.Errorf("coupon code = %q, want %q", gotCode, "SAVE10")
		}
		if got := gotCtx.Value(requestIDKey); got != "req-123" {
			t.Errorf("request ID = %v, want %q", got, "req-123")
		}
		return Coupon{Code: "SAVE10", PercentOff: 10, MinimumSpendYen: 5_000}, nil
	})

	got, err := Calculate(ctx, Order{
		Items: []Item{
			{SKU: "coffee-beans", UnitPriceYen: 6_000, Quantity: 2},
		},
		Membership: MembershipSilver,
		CouponCode: "SAVE10",
		TaxRate:    10,
	}, store)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	want := Breakdown{
		SubtotalYen:       12_000,
		MemberDiscountYen: 360,
		CouponDiscountYen: 1_164,
		ShippingYen:       0,
		TaxYen:            1_047,
		TotalYen:          11_523,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Calculate() = %+v, want %+v", got, want)
	}
}
