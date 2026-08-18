CREATE TABLE payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    driver_id UUID NOT NULL REFERENCES driver_profiles(id) ON DELETE RESTRICT,
    settlement_id UUID REFERENCES settlements(id) ON DELETE RESTRICT,
    status payout_status NOT NULL DEFAULT 'pending',
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency CHAR(3) NOT NULL,
    provider TEXT,
    provider_payout_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
