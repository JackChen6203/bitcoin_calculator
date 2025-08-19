package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
)

func main() {
	fmt.Println("🔍 Simple Supabase Connection Test")
	fmt.Println("=================================")

	// Your connection pooler URL
	databaseURL := "postgresql://postgres.ujlwckhrkscphcnncznz:a126182900@aws-1-us-west-1.pooler.supabase.com:6543/postgres"

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection (single connection, no pool)
	fmt.Println("🔍 Testing single connection...")
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	fmt.Println("✅ Connection successful!")

	// Test ping
	fmt.Println("🔍 Testing ping...")
	err = conn.Ping(ctx)
	if err != nil {
		log.Fatalf("❌ Ping failed: %v", err)
	}
	fmt.Println("✅ Ping successful!")

	// Test database info
	fmt.Println("🔍 Getting database info...")
	var dbName, user string
	err = conn.QueryRow(ctx, "SELECT current_database(), current_user").Scan(&dbName, &user)
	if err != nil {
		fmt.Printf("❌ Failed to get database info: %v\n", err)
	} else {
		fmt.Printf("✅ Database: %s, User: %s\n", dbName, user)
	}

	// Check for required tables
	fmt.Println("🔍 Checking for required tables...")
	
	var keyRangesExists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'key_ranges'
		)
	`).Scan(&keyRangesExists)
	if err != nil {
		fmt.Printf("❌ Failed to check key_ranges: %v\n", err)
	} else if keyRangesExists {
		fmt.Println("✅ key_ranges table EXISTS")
		
		var rowCount int
		err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&rowCount)
		if err != nil {
			fmt.Printf("❌ Failed to count rows: %v\n", err)
		} else {
			fmt.Printf("✅ key_ranges contains %d rows\n", rowCount)
		}
	} else {
		fmt.Println("❌ key_ranges table does NOT exist")
	}

	var foundWalletsExists bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'found_wallets'
		)
	`).Scan(&foundWalletsExists)
	if err != nil {
		fmt.Printf("❌ Failed to check found_wallets: %v\n", err)
	} else if foundWalletsExists {
		fmt.Println("✅ found_wallets table EXISTS")
	} else {
		fmt.Println("❌ found_wallets table does NOT exist")
	}

	fmt.Println("\n🎉 Test completed successfully!")
	fmt.Println("================================")
}