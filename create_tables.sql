-- Bitcoin Calculator Database Schema
-- Create the required tables for the Bitcoin private key scanner

-- Create key_ranges table for work distribution
CREATE TABLE IF NOT EXISTS key_ranges (
    id BIGSERIAL PRIMARY KEY,
    start_key_hex TEXT NOT NULL,
    end_key_hex TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed')),
    worker_id TEXT,
    claimed_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create found_wallets table to store discovered wallets with balances
CREATE TABLE IF NOT EXISTS found_wallets (
    id BIGSERIAL PRIMARY KEY,
    private_key_wif TEXT UNIQUE NOT NULL,
    address TEXT NOT NULL,
    balance_satoshi BIGINT NOT NULL,
    worker_id TEXT NOT NULL,
    discovered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_key_ranges_status ON key_ranges(status);
CREATE INDEX IF NOT EXISTS idx_key_ranges_worker ON key_ranges(worker_id);
CREATE INDEX IF NOT EXISTS idx_found_wallets_address ON found_wallets(address);
CREATE INDEX IF NOT EXISTS idx_found_wallets_discovered ON found_wallets(discovered_at);

-- Display table information
\d key_ranges
\d found_wallets

-- Show table counts
SELECT 'key_ranges' as table_name, COUNT(*) as row_count FROM key_ranges
UNION ALL
SELECT 'found_wallets' as table_name, COUNT(*) as row_count FROM found_wallets;