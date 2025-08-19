package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
)

func main() {
	fmt.Println("🔧 Setting up Supabase Database Schema")
	fmt.Println("=====================================")

	// Your connection pooler URL
	databaseURL := "postgresql://postgres.ujlwckhrkscphcnncznz:a126182900@aws-1-us-west-1.pooler.supabase.com:6543/postgres"

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Connect to database
	fmt.Println("🔍 Connecting to database...")
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer conn.Close(ctx)
	fmt.Println("✅ Connected successfully!")

	// Create key_ranges table
	fmt.Println("🔍 Creating key_ranges table...")
	keyRangesSQL := `
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
	`
	
	_, err = conn.Exec(ctx, keyRangesSQL)
	if err != nil {
		log.Fatalf("❌ Failed to create key_ranges table: %v", err)
	}
	fmt.Println("✅ key_ranges table created!")

	// Create found_wallets table
	fmt.Println("🔍 Creating found_wallets table...")
	foundWalletsSQL := `
		CREATE TABLE IF NOT EXISTS found_wallets (
			id BIGSERIAL PRIMARY KEY,
			private_key_wif TEXT UNIQUE NOT NULL,
			address TEXT NOT NULL,
			balance_satoshi BIGINT NOT NULL,
			worker_id TEXT NOT NULL,
			discovered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`
	
	_, err = conn.Exec(ctx, foundWalletsSQL)
	if err != nil {
		log.Fatalf("❌ Failed to create found_wallets table: %v", err)
	}
	fmt.Println("✅ found_wallets table created!")

	// Create indexes
	fmt.Println("🔍 Creating indexes...")
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_key_ranges_status ON key_ranges(status);",
		"CREATE INDEX IF NOT EXISTS idx_key_ranges_worker ON key_ranges(worker_id);",
		"CREATE INDEX IF NOT EXISTS idx_found_wallets_address ON found_wallets(address);",
		"CREATE INDEX IF NOT EXISTS idx_found_wallets_discovered ON found_wallets(discovered_at);",
	}

	for _, indexSQL := range indexes {
		_, err = conn.Exec(ctx, indexSQL)
		if err != nil {
			fmt.Printf("⚠️ Failed to create index: %v\n", err)
		}
	}
	fmt.Println("✅ Indexes created!")

	// Verify tables exist
	fmt.Println("🔍 Verifying table creation...")
	
	var keyRangesExists, foundWalletsExists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'key_ranges'
		)
	`).Scan(&keyRangesExists)
	
	if err != nil {
		fmt.Printf("❌ Failed to verify key_ranges: %v\n", err)
	} else if keyRangesExists {
		fmt.Println("✅ key_ranges table verified!")
	}

	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'found_wallets'
		)
	`).Scan(&foundWalletsExists)
	
	if err != nil {
		fmt.Printf("❌ Failed to verify found_wallets: %v\n", err)
	} else if foundWalletsExists {
		fmt.Println("✅ found_wallets table verified!")
	}

	// Show table counts
	fmt.Println("🔍 Checking table counts...")
	var keyRangesCount, foundWalletsCount int
	
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&keyRangesCount)
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM found_wallets").Scan(&foundWalletsCount)
	
	fmt.Printf("📊 key_ranges: %d rows\n", keyRangesCount)
	fmt.Printf("📊 found_wallets: %d rows\n", foundWalletsCount)

	fmt.Println("\n🎉 Database setup completed successfully!")
	fmt.Println("Your Bitcoin scanner is now ready to run!")
	fmt.Println("=====================================")
}