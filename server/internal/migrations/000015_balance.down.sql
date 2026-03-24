-- Revert balance pass #1

ALTER TABLE agents ALTER COLUMN credits SET DEFAULT 750;

UPDATE system_control SET income_per_tick = 50 WHERE income_per_tick = 75;
ALTER TABLE system_control ALTER COLUMN income_per_tick SET DEFAULT 50;
