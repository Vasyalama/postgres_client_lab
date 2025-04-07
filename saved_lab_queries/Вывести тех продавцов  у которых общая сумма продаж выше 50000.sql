SELECT saler.id, saler.name, SUM(orders.price) AS total_sales
FROM public.saler
JOIN public.orders ON saler.id = orders.saler_id
GROUP BY saler.id, saler.name
HAVING SUM(orders.price) > 50000
;
