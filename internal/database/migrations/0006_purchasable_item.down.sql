ALTER TABLE purchasable_items
    DROP CONSTRAINT IF EXISTS chk_purchasable_item_plan_positive;

ALTER TABLE purchasable_items
    DROP CONSTRAINT IF EXISTS chk_purchasable_item_name;

ALTER TABLE purchasable_items
    DROP CONSTRAINT IF EXISTS chk_purchasable_item_type;

ALTER TABLE purchasable_items
    DROP CONSTRAINT IF EXISTS uq_purchasable_item_code;