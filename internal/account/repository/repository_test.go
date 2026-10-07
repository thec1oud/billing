package repository

import "testing"

func TestResolvePaymentMethodDetails(t *testing.T) {
	providerCode, paymentTypeCode, err := resolvePaymentMethodDetails("pm_chapa_active")
	if err != nil {
		t.Fatalf("resolvePaymentMethodDetails(pm_chapa_active) returned error: %v", err)
	}
	if providerCode != "chapa" {
		t.Fatalf("providerCode = %q, want %q", providerCode, "chapa")
	}
	if paymentTypeCode != "mobile_money" {
		t.Fatalf("paymentTypeCode = %q, want %q", paymentTypeCode, "mobile_money")
	}

	providerCode, paymentTypeCode, err = resolvePaymentMethodDetails("pm_stripe_card_fail")
	if err != nil {
		t.Fatalf("resolvePaymentMethodDetails(pm_stripe_card_fail) returned error: %v", err)
	}
	if providerCode != "stripe" {
		t.Fatalf("providerCode = %q, want %q", providerCode, "stripe")
	}
	if paymentTypeCode != "card" {
		t.Fatalf("paymentTypeCode = %q, want %q", paymentTypeCode, "card")
	}

	if _, _, err = resolvePaymentMethodDetails("missing-method"); err == nil {
		t.Fatal("resolvePaymentMethodDetails(missing-method) = nil, want error")
	}
}
