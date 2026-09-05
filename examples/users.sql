-- Return the stable OIDC subject, display name, and email in that order.
-- Adapt these columns to the application's existing user schema.
-- Use NULL::text for an absent name or email column.
SELECT id::text, name::text, email::text
FROM public.users
ORDER BY name, id;
