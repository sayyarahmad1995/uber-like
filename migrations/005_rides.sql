CREATE TABLE rides (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rider_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status ride_status NOT NULL DEFAULT 'requested',
    pickup_latitude NUMERIC(9,6) NOT NULL CHECK (pickup_latitude BETWEEN -90 AND 90),
    pickup_longitude NUMERIC(9,6) NOT NULL CHECK (pickup_longitude BETWEEN -180 AND 180),
    dropoff_latitude NUMERIC(9,6) NOT NULL CHECK (dropoff_latitude BETWEEN -90 AND 90),
    dropoff_longitude NUMERIC(9,6) NOT NULL CHECK (dropoff_longitude BETWEEN -180 AND 180),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    bidding_started_at TIMESTAMPTZ,
    reserved_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
