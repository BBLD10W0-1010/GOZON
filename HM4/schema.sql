-- Запускать в отдельной учебной базе данных.
BEGIN;

CREATE TABLE users (
    id   BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    age  INT NOT NULL,
    city TEXT NOT NULL
);

CREATE TABLE orders (
    id      BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id),
    item    TEXT NOT NULL,
    price   INT NOT NULL
);

COMMIT;
