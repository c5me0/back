-- Create "couples" table
CREATE TABLE "couples" (
  "id" uuid NOT NULL,
  "disconnected_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id")
);
-- Create "users" table
CREATE TABLE "users" (
  "id" uuid NOT NULL,
  "phone" character varying NOT NULL,
  "display_name" character varying NULL,
  "pairing_code" character varying NOT NULL,
  "call_alert" boolean NOT NULL DEFAULT true,
  "highlight_alert" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "couple_id" uuid NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "users_couples_members" FOREIGN KEY ("couple_id") REFERENCES "couples" ("id") ON UPDATE NO ACTION ON DELETE SET NULL
);
-- Create index "user_pairing_code" to table: "users"
CREATE UNIQUE INDEX "user_pairing_code" ON "users" ("pairing_code");
-- Create index "user_phone" to table: "users"
CREATE UNIQUE INDEX "user_phone" ON "users" ("phone");
-- Create "calls" table
CREATE TABLE "calls" (
  "id" uuid NOT NULL,
  "status" smallint NOT NULL,
  "transcript_status" smallint NOT NULL,
  "title" character varying NULL,
  "summary" character varying NULL,
  "recording_key" character varying NULL,
  "favorited_by" uuid[] NOT NULL,
  "started_at" timestamptz NULL,
  "ended_at" timestamptz NULL,
  "transcript" jsonb NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "couple_id" uuid NOT NULL,
  "caller_id" uuid NOT NULL,
  "callee_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "calls_couples_calls" FOREIGN KEY ("couple_id") REFERENCES "couples" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "calls_users_incoming_calls" FOREIGN KEY ("callee_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "calls_users_outgoing_calls" FOREIGN KEY ("caller_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "call_transcript_status" to table: "calls"
CREATE INDEX "call_transcript_status" ON "calls" ("transcript_status");
-- Create index "idx_call_list" to table: "calls"
CREATE INDEX "idx_call_list" ON "calls" ("couple_id", "created_at" DESC, "id" DESC);
-- Create index "idx_call_live" to table: "calls"
CREATE UNIQUE INDEX "idx_call_live" ON "calls" ("couple_id") WHERE (status = ANY (ARRAY[0, 1]));
-- Create "call_highlights" table
CREATE TABLE "call_highlights" (
  "id" uuid NOT NULL,
  "offset_seconds" double precision NOT NULL,
  "created_at" timestamptz NOT NULL,
  "call_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "call_highlights_calls_highlights" FOREIGN KEY ("call_id") REFERENCES "calls" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "call_highlights_users_call_highlights" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "callhighlight_call_id_offset_seconds" to table: "call_highlights"
CREATE INDEX "callhighlight_call_id_offset_seconds" ON "call_highlights" ("call_id", "offset_seconds");
-- Create "sessions" table
CREATE TABLE "sessions" (
  "id" uuid NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sessions_users_sessions" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "session_user_id" to table: "sessions"
CREATE INDEX "session_user_id" ON "sessions" ("user_id");
-- Create "devices" table
CREATE TABLE "devices" (
  "id" uuid NOT NULL,
  "platform" smallint NOT NULL,
  "token" character varying NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "session_id" uuid NOT NULL,
  "user_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "devices_sessions_devices" FOREIGN KEY ("session_id") REFERENCES "sessions" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "devices_users_devices" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "device_platform_token" to table: "devices"
CREATE UNIQUE INDEX "device_platform_token" ON "devices" ("platform", "token");
-- Create index "device_user_id" to table: "devices"
CREATE INDEX "device_user_id" ON "devices" ("user_id");
-- Create "photos" table
CREATE TABLE "photos" (
  "id" uuid NOT NULL,
  "status" smallint NOT NULL,
  "content_type" character varying NOT NULL,
  "object_key" character varying NOT NULL,
  "thumbnail_key" character varying NOT NULL,
  "size_bytes" bigint NOT NULL,
  "width" bigint NULL,
  "height" bigint NULL,
  "taken_at" timestamptz NULL,
  "favorited_by" uuid[] NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "call_id" uuid NULL,
  "couple_id" uuid NOT NULL,
  "uploader_id" uuid NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "photos_calls_photos" FOREIGN KEY ("call_id") REFERENCES "calls" ("id") ON UPDATE NO ACTION ON DELETE SET NULL,
  CONSTRAINT "photos_couples_photos" FOREIGN KEY ("couple_id") REFERENCES "couples" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "photos_users_photos" FOREIGN KEY ("uploader_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_photo_list" to table: "photos"
CREATE INDEX "idx_photo_list" ON "photos" ("couple_id", "created_at" DESC, "id" DESC);
-- Create index "photo_call_id" to table: "photos"
CREATE INDEX "photo_call_id" ON "photos" ("call_id");
