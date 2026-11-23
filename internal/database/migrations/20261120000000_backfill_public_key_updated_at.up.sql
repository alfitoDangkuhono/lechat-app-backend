-- Backfill key rotation timestamps for public keys uploaded before the
-- public_key_updated_at column existed. users.updated_at is the closest
-- available approximation at migration time; after this backfill the value
-- only moves when the key itself changes.

UPDATE users
SET public_key_updated_at = updated_at
WHERE public_key IS NOT NULL
  AND public_key_updated_at IS NULL;
