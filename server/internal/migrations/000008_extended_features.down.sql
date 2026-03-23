-- Reverse of 000008_extended_features.up.sql

DROP TABLE IF EXISTS galactic_events;
DROP TABLE IF EXISTS bounties;
DROP TABLE IF EXISTS admin_audit_log;
DROP TABLE IF EXISTS chat_reports;
DROP TABLE IF EXISTS chat_mutes;
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS alliances;

ALTER TABLE ships  DROP COLUMN IF EXISTS cargo_level;
ALTER TABLE ships  DROP COLUMN IF EXISTS engine_level;
ALTER TABLE ships  DROP COLUMN IF EXISTS shield_level;
ALTER TABLE ships  DROP COLUMN IF EXISTS weapon_level;

ALTER TABLE agents DROP COLUMN IF EXISTS respawn_tick;
ALTER TABLE agents DROP COLUMN IF EXISTS death_tick;
ALTER TABLE agents DROP COLUMN IF EXISTS ban_reason;
ALTER TABLE agents DROP COLUMN IF EXISTS banned_at;
ALTER TABLE agents DROP COLUMN IF EXISTS last_override_at;
ALTER TABLE agents DROP COLUMN IF EXISTS last_veto_at;

ALTER TABLE accounts DROP COLUMN IF EXISTS is_admin;
