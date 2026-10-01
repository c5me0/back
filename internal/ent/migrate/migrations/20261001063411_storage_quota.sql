-- Modify "calls" table
ALTER TABLE "calls" ADD COLUMN "recording_bytes" bigint NOT NULL DEFAULT 0;
-- Modify "photos" table
ALTER TABLE "photos" ADD COLUMN "thumbnail_size_bytes" bigint NOT NULL DEFAULT 0;
-- Modify "users" table
ALTER TABLE "users" DROP COLUMN "premium_until", ADD COLUMN "storage_entitlement" character varying NULL, ADD COLUMN "storage_until" timestamptz NULL;
