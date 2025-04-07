SELECT type_of_work
FROM
public.service_schedule

WHERE
status = 'true'
EXCEPT

SELECT type_of_work
FROM 
public.service_schedule
WHERE 
status = 'false';

