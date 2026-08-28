-- Item codes must be unique.
ALTER TABLE purchasable_items
    ADD CONSTRAINT uq_purchasable_item_code
    UNIQUE (item_code);

-- Only supported purchasable item types are allowed.
ALTER TABLE purchasable_items
    ADD CONSTRAINT chk_purchasable_item_type
    CHECK (
        item_type_code IN (
            'PLAN',
            'ONE_TIME_SERVICE',
            'PRODUCT'
        )
    );

-- A purchasable item must have a non-empty name.
ALTER TABLE purchasable_items
    ADD CONSTRAINT chk_purchasable_item_name
    CHECK (length(trim(name)) > 0);

-- If the item references a plan, the plan ID must be positive.
ALTER TABLE purchasable_items
    ADD CONSTRAINT chk_purchasable_item_plan_positive
    CHECK (
        plan_id IS NULL
        OR plan_id > 0
    );