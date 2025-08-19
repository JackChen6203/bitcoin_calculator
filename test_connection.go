package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("🔍 Supabase Connection Test")
	fmt.Println("========================")

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL environment variable is not set")
	}

	fmt.Printf("📋 Database URL: %s...%s\n", databaseURL[:30], databaseURL[len(databaseURL)-20:])

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test 1: Basic connection
	fmt.Println("\n🔍 Test 1: Basic connection")
	dbpool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer dbpool.Close()
	fmt.Println("✅ Connection successful")

	// Test 2: Ping test
	fmt.Println("\n🔍 Test 2: Ping test")
	err = dbpool.Ping(ctx)
	if err != nil {
		log.Fatalf("❌ Ping failed: %v", err)
	}
	fmt.Println("✅ Ping successful")

	// Test 3: Get database info
	fmt.Println("\n🔍 Test 3: Database information")
	var version string
	err = dbpool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		fmt.Printf("❌ Failed to get version: %v\n", err)
	} else {
		fmt.Printf("✅ PostgreSQL Version: %s\n", version[:80]+"...")
	}

	var dbName, schema, user string
	err = dbpool.QueryRow(ctx, "SELECT current_database(), current_schema(), current_user").Scan(&dbName, &schema, &user)
	if err != nil {
		fmt.Printf("❌ Failed to get database info: %v\n", err)
	} else {
		fmt.Printf("✅ Database: %s, Schema: %s, User: %s\n", dbName, schema, user)
	}

	// Test 4: List all tables
	fmt.Println("\n🔍 Test 4: List all tables")
	rows, err := dbpool.Query(ctx, `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		WHERE table_schema = current_schema() 
		ORDER BY table_name
	`)
	if err != nil {
		fmt.Printf("❌ Failed to list tables: %v\n", err)
	} else {
		fmt.Println("📋 Tables in current schema:")
		tableCount := 0
		for rows.Next() {
			var tableName, tableType string
			err = rows.Scan(&tableName, &tableType)
			if err != nil {
				fmt.Printf("  ❌ Error scanning table: %v\n", err)
				continue
			}
			fmt.Printf("  - %s (%s)\n", tableName, tableType)
			tableCount++
		}
		rows.Close()
		fmt.Printf("✅ Total tables: %d\n", tableCount)
	}

	// Test 5: Check specific tables
	fmt.Println("\n🔍 Test 5: Check required tables")
	
	// Check key_ranges table
	var keyRangesExists bool
	err = dbpool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = current_schema() 
			AND table_name = 'key_ranges'
		)
	`).Scan(&keyRangesExists)
	if err != nil {
		fmt.Printf("❌ Failed to check key_ranges: %v\n", err)
	} else if keyRangesExists {
		fmt.Println("✅ key_ranges table EXISTS")
		
		// Count rows
		var rowCount int
		err = dbpool.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&rowCount)
		if err != nil {
			fmt.Printf("❌ Failed to count key_ranges rows: %v\n", err)
		} else {
			fmt.Printf("✅ key_ranges contains %d rows\n", rowCount)
			
			if rowCount > 0 {
				// Check status distribution
				statusRows, err := dbpool.Query(ctx, "SELECT status, COUNT(*) FROM key_ranges GROUP BY status ORDER BY status")
				if err != nil {
					fmt.Printf("❌ Failed to get status distribution: %v\n", err)
				} else {
					fmt.Println("📊 Status distribution:")
					for statusRows.Next() {
						var status string
						var count int
						err = statusRows.Scan(&status, &count)
						if err != nil {
							fmt.Printf("  ❌ Error scanning status: %v\n", err)
							continue
						}
						fmt.Printf("  - %s: %d\n", status, count)
					}
					statusRows.Close()
				}
			}
		}
	} else {
		fmt.Println("❌ key_ranges table does NOT exist")
	}

	// Check found_wallets table
	var foundWalletsExists bool
	err = dbpool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = current_schema() 
			AND table_name = 'found_wallets'
		)
	`).Scan(&foundWalletsExists)
	if err != nil {
		fmt.Printf("❌ Failed to check found_wallets: %v\n", err)
	} else if foundWalletsExists {
		fmt.Println("✅ found_wallets table EXISTS")
		
		var walletCount int
		err = dbpool.QueryRow(ctx, "SELECT COUNT(*) FROM found_wallets").Scan(&walletCount)
		if err != nil {
			fmt.Printf("❌ Failed to count found_wallets: %v\n", err)
		} else {
			fmt.Printf("✅ found_wallets contains %d wallets\n", walletCount)
		}
	} else {
		fmt.Println("❌ found_wallets table does NOT exist")
	}

	// Test 6: Test the claimWorkUnit query
	fmt.Println("\n🔍 Test 6: Test claimWorkUnit query")
	if keyRangesExists {
		testQuery := `
			SELECT id, start_key_hex, end_key_hex
			FROM key_ranges
			WHERE status = 'pending'
			ORDER BY id
			LIMIT 1
		`
		var id int64
		var startKey, endKey string
		err = dbpool.QueryRow(ctx, testQuery).Scan(&id, &startKey, &endKey)
		if err != nil {
			if err.Error() == "no rows in result set" {
				fmt.Println("⚠️  No pending work units available")
			} else {
				fmt.Printf("❌ claimWorkUnit query failed: %v\n", err)
			}
		} else {
			fmt.Printf("✅ Found pending work unit: ID=%d, Range=%s-%s\n", id, startKey, endKey)
		}
	}

	fmt.Println("\n🎉 Connection test completed!")
	fmt.Println("========================")
}