-- Modify "couples" table
ALTER TABLE "couples" ADD COLUMN "user_ids" uuid[] NOT NULL DEFAULT '{}', ADD COLUMN "restored_at" timestamptz NULL, ADD COLUMN "restore_transaction_id" character varying NULL;
-- Create index "idx_couple_restore_transaction" to table: "couples"
CREATE UNIQUE INDEX "idx_couple_restore_transaction" ON "couples" ("restore_transaction_id");
-- Modify "users" table
ALTER TABLE "users" ADD COLUMN "premium_until" timestamptz NULL, ADD COLUMN "restore_transaction_ids" text[] NOT NULL DEFAULT '{}', ADD COLUMN "purchases_synced_at" timestamptz NULL;
