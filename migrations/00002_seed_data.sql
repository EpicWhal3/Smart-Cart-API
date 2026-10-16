-- +goose Up
-- +goose StatementBegin
INSERT INTO catalog_items (sku_id, title, category_id, current_price, old_price, stock_quantity)
VALUES
    -- Категория 1: клавиатуры
    (101, 'Механическая клавиатура Keychron K2', 1, 115000, 135000, 3),
    (102, 'Механическая клавиатура Ducky One 3', 1, 149900, NULL, 50),
    (103, 'Механическая клавиатура Logitech MX Mechanical', 1, 179000, 199000, 2),
    (104, 'Мембранная клавиатура Logitech K120', 1, 99000, NULL, 0),
    -- Категория 2: мыши
    (201, 'Беспроводная мышь Logitech G Pro X Superlight', 2, 119000, 139000, 50),
    (202, 'Проводная мышь Razer DeathAdder V3', 2, 79000, NULL, 2),
    (203, 'Вертикальная мышь Logitech Lift', 2, 69000, 89000, 5),
    (205, 'Беспроводная мышь Logitech MX Master 3S', 2, 99000, 119000, 0),
    (206, 'Беспроводная мышь Razer Pro Click', 2, 89000, NULL, 50),
    -- Категория 3: мониторы
    (301, 'Монитор Dell UltraSharp U2723QE', 3, 549000, 599000, 4),
    (302, 'Монитор LG UltraGear 27GP850', 3, 399000, NULL, 0),
    (303, 'Монитор Samsung Odyssey G7', 3, 459000, 529000, 50),
    -- Категория 4: наушники
    (401, 'Наушники Sony WH-1000XM5', 4, 299000, 349000, 2),
    (402, 'Наушники Apple AirPods Pro 2', 4, 249000, NULL, 50),
    (403, 'Наушники Bose QuietComfort', 4, 279000, 319000, 0) ON CONFLICT (sku_id) DO NOTHING;

INSERT INTO promo_codes (code, discount_percent, min_order_amount, is_active)
VALUES ('OZON10', 10, 200000, TRUE),
       ('SAVE15', 15, 500000, TRUE),
       ('WELCOME5', 5, 0, TRUE),
       ('OLD20', 20, 100000, FALSE) ON CONFLICT (code) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE
FROM promo_codes
WHERE code IN ('OZON10', 'SAVE15', 'WELCOME5', 'OLD20');
DELETE
FROM catalog_items
WHERE sku_id BETWEEN 100 AND 403;
-- +goose StatementEnd
