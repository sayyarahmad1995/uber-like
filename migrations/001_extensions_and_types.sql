CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_status AS ENUM ('active', 'suspended', 'deactivated');
CREATE TYPE driver_status AS ENUM ('active', 'suspended', 'deactivated');
CREATE TYPE vehicle_status AS ENUM ('active', 'inactive', 'suspended');
CREATE TYPE ride_status AS ENUM ('requested', 'bidding', 'reserved', 'assigned', 'driver_arrived', 'in_progress', 'completed', 'cancelled');
CREATE TYPE bid_status AS ENUM ('submitted', 'active', 'withdrawn', 'rejected', 'selected', 'expired');
CREATE TYPE reservation_status AS ENUM ('pending', 'confirmed', 'expired', 'cancelled');
CREATE TYPE assignment_status AS ENUM ('assigned', 'driver_arrived', 'in_progress', 'completed', 'cancelled', 'released');
CREATE TYPE payment_status AS ENUM ('pending', 'authorized', 'captured', 'failed', 'refunded', 'partially_refunded', 'cancelled');
CREATE TYPE settlement_status AS ENUM ('pending', 'calculated', 'finalized', 'reversed');
CREATE TYPE payout_status AS ENUM ('pending', 'submitted', 'processing', 'paid', 'failed', 'cancelled');
