SELECT * FROM public.payment
WHERE process = 'Карта'
AND sum > 10000;