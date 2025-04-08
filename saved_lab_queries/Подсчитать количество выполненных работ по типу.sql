SELECT
 type_of_work,
 COUNT(*) AS total_services
FROM public.service_schedule
GROUP BY type_of_work
ORDER BY total_services DESC;
