package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func main() {
	fmt.Println("🔍 Supabase Connection Pooler Test")
	fmt.Println("==================================")

	// Your connection pooler URL
	databaseURL := "postgresql://postgres.ujlwckhrkscphcnncznz:a126182900@aws-1-us-west-1.pooler.supabase.com:6543/postgres"
	
	fmt.Printf("📋 Testing connection pooler URL\n")
	fmt.Printf("   Host: aws-1-us-west-1.pooler.supabase.com\n")
	fmt.Printf("   Port: 6543 (connection pooler)\n")
	fmt.Printf("   User: postgres.ujlwckhrkscphcnncznz\n")
	fmt.Printf("   Database: postgres\n\n")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test 1: Basic pooler connection
	fmt.Println("🔍 Test 1: Connection pooler access")
	start := time.Now()
	dbpool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect via pooler: %v", err)
	}
	defer dbpool.Close()
	connectionTime := time.Since(start)
	fmt.Printf("✅ Pooler connection successful (took %v)\n", connectionTime)

	// Test 2: Pool configuration
	fmt.Println("\n🔍 Test 2: Connection pool configuration")
	config := dbpool.Config()
	fmt.Printf("✅ Max connections: %d\n", config.MaxConns)
	fmt.Printf("✅ Min connections: %d\n", config.MinConns)

	// Test 3: Pool statistics
	fmt.Println("\n🔍 Test 3: Pool statistics")
	stats := dbpool.Stat()
	fmt.Printf("✅ Total connections: %d\n", stats.TotalConns())
	fmt.Printf("✅ Idle connections: %d\n", stats.IdleConns())
	fmt.Printf("✅ Acquired connections: %d\n", stats.AcquiredConns())

	// Test 4: Ping test with timing
	fmt.Println("\n🔍 Test 4: Ping test with timing")
	start = time.Now()
	err = dbpool.Ping(ctx)
	pingTime := time.Since(start)
	if err != nil {
		log.Fatalf("❌ Ping failed: %v", err)
	}
	fmt.Printf("✅ Ping successful (took %v)\n", pingTime)

	// Test 5: Database info
	fmt.Println("\n🔍 Test 5: Database information")
	var version, dbName, schema, user string
	var serverAddr, serverPort string

	start = time.Now()
	err = dbpool.QueryRow(ctx, "SELECT version()").Scan(&version)
	queryTime := time.Since(start)
	if err != nil {
		fmt.Printf("❌ Failed to get version: %v\n", err)
	} else {
		fmt.Printf("✅ PostgreSQL Version (took %v): %s\n", queryTime, version[:60]+"...")
	}

	err = dbpool.QueryRow(ctx, "SELECT current_database(), current_schema(), current_user").Scan(&dbName, &schema, &user)
	if err != nil {
		fmt.Printf("❌ Failed to get database info: %v\n", err)
	} else {
		fmt.Printf("✅ Database: %s, Schema: %s, User: %s\n", dbName, schema, user)
	}

	// Test 6: Connection info (pooler specific)
	fmt.Println("\n🔍 Test 6: Connection details")
	err = dbpool.QueryRow(ctx, "SELECT inet_server_addr(), inet_server_port()").Scan(&serverAddr, &serverPort)
	if err != nil {
		fmt.Printf("❌ Failed to get connection info: %v\n", err)
	} else {
		fmt.Printf("✅ Connected to: %s:%s\n", serverAddr, serverPort)
	}

	// Test 7: Check for tables
	fmt.Println("\n🔍 Test 7: Check required tables")
	
	// Check key_ranges
	var keyRangesExists bool
	err = dbpool.QueryRow(ctx, `
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
		err = dbpool.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&rowCount)
		if err != nil {
			fmt.Printf("❌ Failed to count rows: %v\n", err)
		} else {
			fmt.Printf("✅ key_ranges contains %d rows\n", rowCount)
		}
	} else {
		fmt.Println("❌ key_ranges table does NOT exist")
	}

	// Check found_wallets  
	var foundWalletsExists bool
	err = dbpool.QueryRow(ctx, `
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

	// Test 8: Performance test with multiple queries
	fmt.Println("\n🔍 Test 8: Performance test (10 queries)")
	start = time.Now()
	for i := 0; i < 10; i++ {
		var result int
		err = dbpool.QueryRow(ctx, "SELECT $1", i).Scan(&result)
		if err != nil {
			fmt.Printf("❌ Query %d failed: %v\n", i+1, err)
			break
		}
	}
	totalTime := time.Since(start)
	fmt.Printf("✅ 10 queries completed in %v (avg: %v per query)\n", totalTime, totalTime/10)

	// Final pool stats
	fmt.Println("\n📊 Final pool statistics:")
	finalStats := dbpool.Stat()
	fmt.Printf("  Total connections: %d\n", finalStats.TotalConns())
	fmt.Printf("  Idle connections: %d\n", finalStats.IdleConns())
	fmt.Printf("  Acquired connections: %d\n", finalStats.AcquiredConns())

	fmt.Println("\n🎉 Connection pooler test completed successfully!")
	fmt.Println("==================================")
}