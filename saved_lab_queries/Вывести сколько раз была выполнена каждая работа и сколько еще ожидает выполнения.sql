SELECT
 type_of_work,
COUNT(status = true) as completed_services,
COUNT(status = false) as pending_services
FROM public.service_schedule
GROUP BY type_of_work


