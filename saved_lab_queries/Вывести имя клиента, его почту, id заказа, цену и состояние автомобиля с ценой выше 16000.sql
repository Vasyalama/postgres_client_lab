SELECT
 client.name AS client_name,
 client.email,
 orders.id AS order_id,
 orders.price,
 orders.condition
FROM public.orders
JOIN public.client ON orders.client_id = client.id
WHERE orders.price > 16000
ORDER BY orders.price DESC;
