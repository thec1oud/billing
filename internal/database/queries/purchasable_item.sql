-- name: CreatePurchasableItem :one
INSERT INTO purchasable_items (
    item_code,
    item_type_code,
    name,
    description,
    plan_id,
    is_active,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING
    item_id,
    item_code,
    item_type_code,
    name,
    description,
    plan_id,
    is_active,
    metadata,
    created_at;


-- name: GetPurchasableItemByID :one
SELECT
    item_id,
    item_code,
    item_type_code,
    name,
    description,
    plan_id,
    is_active,
    metadata,
    created_at
FROM purchasable_items
WHERE item_id = $1;


-- name: GetPurchasableItemByCode :one
SELECT
    item_id,
    item_code,
    item_type_code,
    name,
    description,
    plan_id,
    is_active,
    metadata,
    created_at
FROM purchasable_items
WHERE item_code = $1;

-- name: GetPurchasableItemByPlanID :one
SELECT
    item_id,
    item_code,
    item_type_code,
    name,
    description,
    plan_id,
    is_active,
    metadata,
    created_at
FROM purchasable_items
WHERE plan_id = $1
ORDER BY item_id
LIMIT 1;

-- name: ListPurchasableItems :many
SELECT
    item_id,
    item_code,
    item_type_code,
    name,
    description,
    plan_id,
    is_active,
    metadata,
    created_at
FROM purchasable_items
ORDER BY item_id DESC;