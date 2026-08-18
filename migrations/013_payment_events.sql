CREATE TABLE payment_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payment_id UUID NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    event_type TEXT NOT NULL,
    provider_event_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB
);
