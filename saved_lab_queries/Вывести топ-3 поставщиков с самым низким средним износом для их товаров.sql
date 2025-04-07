SELECT 
    supplier.name, 
    AVG(auto_supplier.wear) AS avg_wear
FROM public.auto_supplier AS auto_supplier
JOIN public.supplier AS supplier
    ON auto_supplier.supplier_id = supplier.id
GROUP BY supplier.name
ORDER BY avg_wear ASC
LIMIT 3;


