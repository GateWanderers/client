-- Phase 12: Admin Dashboard schema additions

-- Mark accounts as admins.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- Agent ban support.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS banned_at   TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS ban_reason  TEXT;

-- Audit log for all admin actions.
CREATE TABLE IF NOT EXISTS admin_audit_log (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_account_id UUID        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    action           TEXT        NOT NULL,
    target_id        TEXT,
    details          JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_created_at ON admin_audit_log(created_at DESC);
