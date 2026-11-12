-- 1. Долгожитель: возвращаем всех москвичей с максимальным возрастом.
SELECT name, age
FROM users
WHERE city = 'Москва'
  AND age = (SELECT MAX(age) FROM users WHERE city = 'Москва')
ORDER BY name, id;

-- 2. Перепись населения: HAVING отбирает группы по численности.
SELECT city, COUNT(*) AS user_count
FROM users
GROUP BY city
HAVING COUNT(*) > 1
ORDER BY user_count DESC, city ASC;

-- 3. Учёт трат: LEFT JOIN сохраняет пользователей без заказов.
SELECT u.name, COALESCE(SUM(o.price), 0) AS total_spent
FROM users AS u
LEFT JOIN orders AS o ON o.user_id = u.id
GROUP BY u.id, u.name
ORDER BY total_spent DESC, u.id ASC;

-- 4. Любители музыки: EXISTS не размножает пользователя по заказам.
SELECT DISTINCT u.name
FROM users AS u
WHERE EXISTS (
    SELECT 1
    FROM orders AS o
    WHERE o.user_id = u.id
      AND o.item = 'наушники'
)
ORDER BY u.name;

-- 5. Бюджетный выбор: сохраняем каждый заказ с минимальной ценой выше порога.
SELECT item, price
FROM orders
WHERE price = (SELECT MIN(price) FROM orders WHERE price > 10000)
ORDER BY item, id;
