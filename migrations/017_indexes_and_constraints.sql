CREATE INDEX idx_rides_rider_created_at ON rides (rider_id, created_at DESC);
CREATE INDEX idx_rides_status_created_at ON rides (status, created_at DESC);
CREATE INDEX idx_bids_ride_created_at ON bids (ride_id, created_at DESC);
CREATE INDEX idx_bids_driver_created_at ON bids (driver_id, created_at DESC);
CREATE INDEX idx_reservations_ride ON reservations (ride_id);
CREATE INDEX idx_reservations_driver ON reservations (driver_id);
CREATE INDEX idx_assignments_ride ON assignments (ride_id);
CREATE INDEX idx_assignments_driver ON assignments (driver_id);
CREATE INDEX idx_ride_events_ride_occurred_at ON ride_events (ride_id, occurred_at DESC);
CREATE INDEX idx_bid_events_bid_occurred_at ON bid_events (bid_id, occurred_at DESC);
CREATE INDEX idx_payments_ride ON payments (ride_id);
CREATE INDEX idx_payment_events_payment_occurred_at ON payment_events (payment_id, occurred_at DESC);
CREATE INDEX idx_settlements_ride ON settlements (ride_id);
CREATE INDEX idx_payouts_driver_created_at ON payouts (driver_id, created_at DESC);
CREATE INDEX idx_outbox_unpublished ON outbox_events (created_at ASC) WHERE published_at IS NULL;

CREATE UNIQUE INDEX ux_bids_active_ride_driver
    ON bids (ride_id, driver_id)
    WHERE status IN ('submitted', 'active');

CREATE UNIQUE INDEX ux_reservations_active_ride
    ON reservations (ride_id)
    WHERE status IN ('pending', 'confirmed');

CREATE UNIQUE INDEX ux_assignments_active_ride
    ON assignments (ride_id)
    WHERE status IN ('assigned', 'driver_arrived', 'in_progress');

CREATE UNIQUE INDEX ux_assignments_active_driver
    ON assignments (driver_id)
    WHERE status IN ('assigned', 'driver_arrived', 'in_progress');

CREATE UNIQUE INDEX ux_payments_provider_event_id
    ON payment_events (provider_event_id)
    WHERE provider_event_id IS NOT NULL;

CREATE UNIQUE INDEX ux_payments_provider_payment_id
    ON payments (provider, provider_payment_id)
    WHERE provider IS NOT NULL AND provider_payment_id IS NOT NULL;

CREATE UNIQUE INDEX ux_payouts_provider_payout_id
    ON payouts (provider, provider_payout_id)
    WHERE provider IS NOT NULL AND provider_payout_id IS NOT NULL;
