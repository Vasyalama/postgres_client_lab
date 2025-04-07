SELECT brand
FROM public.car
GROUP BY brand
ORDER BY AVG(price) DESC
LIMIT 3;

