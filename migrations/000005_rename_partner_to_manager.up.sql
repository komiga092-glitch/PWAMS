-- The SRS specifies an NGO-level "Manager" role (one active Manager per
-- NGO). The role was originally seeded as "Partner". Rename it in place so
-- existing users keep their role assignment (role_id is unchanged).
UPDATE roles SET name = 'Manager' WHERE name = 'Partner';
