-- Balance pass #1 (pre-playtest defaults)

-- Raise starting credits for new agents.
ALTER TABLE agents ALTER COLUMN credits SET DEFAULT 1000;

-- Update existing system income (50 → 75 cr/tick).
UPDATE system_control SET income_per_tick = 75 WHERE income_per_tick = 50;
ALTER TABLE system_control ALTER COLUMN income_per_tick SET DEFAULT 75;
