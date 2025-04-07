SELECT 
    supplier.name,
    COUNT(supplier.id) AS parts_in_range
FROM 
public.supplier AS supplier
JOIN 
public.auto_supplier AS auto_supplier
    ON supplier.id = auto_supplier.supplier_id
WHERE auto_supplier.wear BETWEEN 20 AND 50
GROUP BY supplier.name
ORDER BY parts_in_range DESC;



