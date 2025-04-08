SELECT
 orders.id AS order_id,
 client.name AS client_name,
 orders.price,
 orders.condition,
 orders.credit
FROM public.orders
JOIN public.client ON orders.client_id = client.id
WHERE orders.credit = true
ORDER BY orders.price DESC;