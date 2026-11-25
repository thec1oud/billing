package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/shared/apperr"
)

type ItemTypeCode string

const (
	ItemTypePlan           ItemTypeCode = "PLAN"
	ItemTypeOneTimeService ItemTypeCode = "ONE_TIME_SERVICE"
	ItemTypeProduct        ItemTypeCode = "PRODUCT"
)

func (t ItemTypeCode) Valid() bool {
	switch t {
	case ItemTypePlan,
		ItemTypeOneTimeService,
		ItemTypeProduct:
		return true
	default:
		return false
	}
}

type PurchasableItem struct {
	ID           int64           `json:"item_id"`
	ItemCode     string          `json:"item_code"`
	ItemTypeCode ItemTypeCode    `json:"item_type_code"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	PlanID       *int64          `json:"plan_id,omitempty"`
	IsActive     bool            `json:"is_active"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (i PurchasableItem) Validate() error {
	if i.ItemCode == "" {
		return apperr.Validation("required_field", "item code is required")
	}

	if !i.ItemTypeCode.Valid() {
		return apperr.Validation(
			"invalid_item_type",
			fmt.Sprintf("unsupported item type %q", i.ItemTypeCode),
		)
	}

	if i.Name == "" {
		return apperr.Validation("required_field", "item name is required")
	}

	return nil
}
