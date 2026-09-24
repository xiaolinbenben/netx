CREATE INDEX IF NOT EXISTS idx_codes_plan_status ON codes (plan, status);
CREATE INDEX IF NOT EXISTS idx_payment_orders_pending_created ON payment_orders (status, created_at);
