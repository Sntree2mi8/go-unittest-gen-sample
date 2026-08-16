// Package checkout provides order pricing rules for an online store.
package checkout

import (
	"context"
	"errors"
	"fmt"
)

const (
	standardShippingYen int64 = 600
	freeShippingYen     int64 = 10_000
)

var (
	ErrEmptyOrder      = errors.New("order must contain at least one item")
	ErrInvalidItem     = errors.New("item is invalid")
	ErrDuplicateSKU    = errors.New("duplicate SKU")
	ErrUnknownMember   = errors.New("unknown membership level")
	ErrInvalidTaxRate  = errors.New("tax rate must be between 0 and 100")
	ErrCouponNotFound  = errors.New("coupon not found")
	ErrCouponNotUsable = errors.New("coupon cannot be used")
)

// Membership identifies the customer's membership level.
type Membership string

const (
	MembershipGuest  Membership = "guest"
	MembershipSilver Membership = "silver"
	MembershipGold   Membership = "gold"
)

// Item is a single kind of product in an order. UnitPriceYen is tax-exclusive.
type Item struct {
	SKU          string
	UnitPriceYen int64
	Quantity     int
}

// Coupon is returned by CouponStore. Exactly one of PercentOff and AmountOffYen
// must be positive.
type Coupon struct {
	Code            string
	PercentOff      int
	AmountOffYen    int64
	MinimumSpendYen int64
	MembersOnly     bool
}

// CouponStore is deliberately small so callers can replace it with a test double.
type CouponStore interface {
	Find(ctx context.Context, code string) (Coupon, error)
}

// Order contains the inputs used to calculate a total.
type Order struct {
	Items        []Item
	Membership   Membership
	CouponCode   string
	TaxRate      int // whole percent, for example 10 means 10%
	RemoteIsland bool
}

// Breakdown explains how TotalYen was calculated.
type Breakdown struct {
	SubtotalYen       int64
	MemberDiscountYen int64
	CouponDiscountYen int64
	ShippingYen       int64
	TaxYen            int64
	TotalYen          int64
}

// Calculate returns the amount payable for an order.
//
// Discounts are truncated to whole yen. Tax is calculated after discounts and
// before shipping, then rounded down. Gold members receive free standard
// shipping; otherwise standard shipping is free when the discounted merchandise
// total is at least 10,000 yen. Remote-island delivery adds 1,200 yen even when
// standard shipping is free.
func Calculate(ctx context.Context, order Order, coupons CouponStore) (Breakdown, error) {
	if err := validate(order); err != nil {
		return Breakdown{}, err
	}

	subtotal := int64(0)
	for _, item := range order.Items {
		subtotal += item.UnitPriceYen * int64(item.Quantity)
	}

	memberDiscount := subtotal * int64(memberDiscountPercent(order.Membership)) / 100
	afterMemberDiscount := subtotal - memberDiscount

	couponDiscount := int64(0)
	if order.CouponCode != "" {
		if coupons == nil {
			return Breakdown{}, errors.New("coupon store is required when a coupon code is supplied")
		}

		coupon, err := coupons.Find(ctx, order.CouponCode)
		if err != nil {
			return Breakdown{}, fmt.Errorf("find coupon %q: %w", order.CouponCode, err)
		}
		if coupon.Code == "" {
			return Breakdown{}, fmt.Errorf("%w: %s", ErrCouponNotFound, order.CouponCode)
		}
		if err := validateCoupon(coupon, afterMemberDiscount, order.Membership); err != nil {
			return Breakdown{}, fmt.Errorf("%w: %s", err, order.CouponCode)
		}

		if coupon.PercentOff > 0 {
			couponDiscount = afterMemberDiscount * int64(coupon.PercentOff) / 100
		} else {
			couponDiscount = min(coupon.AmountOffYen, afterMemberDiscount)
		}
	}

	merchandiseTotal := afterMemberDiscount - couponDiscount
	shipping := standardShippingYen
	if order.Membership == MembershipGold || merchandiseTotal >= freeShippingYen {
		shipping = 0
	}
	if order.RemoteIsland {
		shipping += 1_200
	}

	tax := merchandiseTotal * int64(order.TaxRate) / 100
	return Breakdown{
		SubtotalYen:       subtotal,
		MemberDiscountYen: memberDiscount,
		CouponDiscountYen: couponDiscount,
		ShippingYen:       shipping,
		TaxYen:            tax,
		TotalYen:          merchandiseTotal + shipping + tax,
	}, nil
}

func validate(order Order) error {
	if len(order.Items) == 0 {
		return ErrEmptyOrder
	}
	if order.TaxRate < 0 || order.TaxRate > 100 {
		return ErrInvalidTaxRate
	}
	if order.Membership != MembershipGuest && order.Membership != MembershipSilver && order.Membership != MembershipGold {
		return fmt.Errorf("%w: %q", ErrUnknownMember, order.Membership)
	}

	seen := make(map[string]struct{}, len(order.Items))
	for i, item := range order.Items {
		if item.SKU == "" || item.UnitPriceYen < 0 || item.Quantity <= 0 {
			return fmt.Errorf("%w at index %d", ErrInvalidItem, i)
		}
		if _, ok := seen[item.SKU]; ok {
			return fmt.Errorf("%w: %s", ErrDuplicateSKU, item.SKU)
		}
		seen[item.SKU] = struct{}{}
	}
	return nil
}

func memberDiscountPercent(membership Membership) int {
	switch membership {
	case MembershipSilver:
		return 3
	case MembershipGold:
		return 5
	default:
		return 0
	}
}

func validateCoupon(coupon Coupon, spend int64, membership Membership) error {
	validPercent := coupon.PercentOff > 0 && coupon.PercentOff <= 100 && coupon.AmountOffYen == 0
	validAmount := coupon.AmountOffYen > 0 && coupon.PercentOff == 0
	if !validPercent && !validAmount {
		return ErrCouponNotUsable
	}
	if spend < coupon.MinimumSpendYen {
		return ErrCouponNotUsable
	}
	if coupon.MembersOnly && membership == MembershipGuest {
		return ErrCouponNotUsable
	}
	return nil
}
