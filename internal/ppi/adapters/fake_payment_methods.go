package adapters

type MethodType string

const (
	MethodTypeCard        MethodType = "CARD"
	MethodTypeMobileMoney MethodType = "MOBILE_MONEY"
)

type PaymentMethod struct {
	ID           string
	ProviderID   string
	MethodType   MethodType
	DisplayLabel string
	IsDefault    bool
}

var MockPaymentMethods = map[string]PaymentMethod{
	"pm_chapa_active": {
		ID:           "pm_chapa_active",
		ProviderID:   "prov_chapa",
		MethodType:   MethodTypeMobileMoney,
		DisplayLabel: "Telebirr ending 0911",
		IsDefault:    true,
	},
	"pm_stripe_card_fail": {
		ID:           "pm_stripe_card_fail",
		ProviderID:   "prov_stripe",
		MethodType:   MethodTypeCard,
		DisplayLabel: "Visa ending 0002",
	},
}
