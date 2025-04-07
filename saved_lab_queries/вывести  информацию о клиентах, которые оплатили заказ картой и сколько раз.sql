SELECT client.name, COUNT(orders.id) AS total_orders
FROM public.client
JOIN public.orders ON client.id = orders.client_id
JOIN public.payment ON orders.id = payment.orders_id
WHERE   payment.process = 'Cash' 
GROUP BY client.name
ORDER BY  total_orders DESC; 




