SELECT name, email
FROM public.client
WHERE id IN (
    SELECT client_id FROM public.orders
    WHERE CAST(price AS NUMERIC) > (SELECT AVG(CAST(price AS NUMERIC)) FROM public.orders)
);




