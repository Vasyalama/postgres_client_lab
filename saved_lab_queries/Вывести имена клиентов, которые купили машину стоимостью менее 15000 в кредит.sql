SELECT orders.id, client.name AS client_name, orders.price
FROM public.orders
JOIN public.client ON orders.client_id = client.id
WHERE orders.credit = 'true'

EXCEPT

SELECT orders.id, client.name AS client_name, orders.price
FROM public.orders
JOIN public.client ON orders.client_id = client.id
WHERE orders.price > 15000;
