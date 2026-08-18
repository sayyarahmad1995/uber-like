CREATE TABLE settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ride_id UUID NOT NULL REFERENCES rides(id) ON DELETE RESTRICT,
    status settlement_status NOT NULL DEFAULT 'pending',
    gross_amount_minor BIGINT NOT NULL CHECK (gross_amount_minor >= 0),
    platform_fee_minor BIGINT NOT NULL CHECK (platform_fee_minor >= 0),
    driver_amount_minor BIGINT NOT NULL CHECK (driver_amount_minor >= 0),
    currency CHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (gross_amount_minor = platform_fee_minor + driver_amount_minor)
);
