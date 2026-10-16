-- +goose Up
-- +goose StatementBegin

-- Таблица корзин: одна активная корзина на пользователя.
CREATE TABLE IF NOT EXISTS carts
(
    user_id
    BIGINT
    PRIMARY
    KEY,
    promo_code
    VARCHAR
(
    64
) NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now
(
)
    );

-- Таблица позиций корзины.
CREATE TABLE IF NOT EXISTS cart_items
(
    user_id
    BIGINT
    NOT
    NULL
    REFERENCES
    carts
(
    user_id
) ON DELETE CASCADE,
    sku_id BIGINT NOT NULL,
    quantity INT NOT NULL CHECK
(
    quantity >
    0
),
    is_selected BOOLEAN NOT NULL DEFAULT TRUE,
    price_at_addition BIGINT NOT NULL,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now
(
),
    PRIMARY KEY
(
    user_id,
    sku_id
)
    );

-- Встроенный каталог товаров (для самодостаточности проекта).
CREATE TABLE IF NOT EXISTS catalog_items
(
    sku_id
    BIGINT
    PRIMARY
    KEY,
    title
    TEXT
    NOT
    NULL,
    category_id
    INT
    NOT
    NULL,
    current_price
    BIGINT
    NOT
    NULL,
    old_price
    BIGINT
    NULL,
    stock_quantity
    INT
    NOT
    NULL
    CHECK
(
    stock_quantity
    >=
    0
)
    );

-- Промокоды.
CREATE TABLE IF NOT EXISTS promo_codes
(
    code
    VARCHAR
(
    64
) PRIMARY KEY,
    discount_percent INT NOT NULL,
    min_order_amount BIGINT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
    );

CREATE INDEX IF NOT EXISTS idx_catalog_items_category ON catalog_items (category_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS promo_codes;
DROP TABLE IF EXISTS catalog_items;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
-- +goose StatementEnd
