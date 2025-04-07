SELECT saler.id, saler.name
FROM 
    public.saler AS saler
JOIN (
    SELECT saler_id
    FROM public.orders
    GROUP BY saler_id
    HAVING MAX(price) > 16000
) AS filtered_orders
ON saler.id = filtered_orders.saler_id;
