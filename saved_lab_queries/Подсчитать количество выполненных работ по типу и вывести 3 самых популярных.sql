SELECT type_of_work, 
COUNT(*) AS total_jobs
FROM 
public.service_schedule
GROUP BY
type_of_work
ORDER BY 
total_jobs DESC
LIMIT 3;

