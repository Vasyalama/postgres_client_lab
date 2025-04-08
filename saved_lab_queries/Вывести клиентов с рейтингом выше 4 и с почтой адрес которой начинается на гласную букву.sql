SELECT *
FROM public.client
WHERE rating > 4
AND email ~* '^[AEIOU]';