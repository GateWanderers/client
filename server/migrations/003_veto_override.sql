-- +migrate Up
ALTER TABLE agents
  ADD COLUMN IF NOT EXISTS last_veto_at     TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_override_at TIMESTAMPTZ;
