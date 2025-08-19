package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
)

func testWithFreshConnection(testName string, query string) {
	fmt.Printf("🔍 %s\n", testName)
	
	// Create fresh context and connection for each test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := "postgresql://postgres.ujlwckhrkscphcnncznz:a126182900@aws-1-us-west-1.pooler.supabase.com:6543/postgres"
	
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		return
	}
	defer conn.Close(ctx)

	// Execute query
	rows, err := conn.Query(ctx, query)
	if err != nil {
		fmt.Printf("❌ Query failed: %v\n", err)
		return
	}
	defer rows.Close()

	// Process results
	columnDescriptions := rows.FieldDescriptions()
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			fmt.Printf("❌ Scan failed: %v\n", err)
			continue
		}
		
		for i, value := range values {
			fmt.Printf("✅ %s: %v\n", columnDescriptions[i].Name, value)
		}
	}
}

func main() {
	fmt.Println("🔍 Fresh Connection Supabase Test")
	fmt.Println("=================================")

	// Test 1: Database info
	testWithFreshConnection("Database info", "SELECT current_database(), current_user")
	
	// Test 2: Check tables
	testWithFreshConnection("List all tables", `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		WHERE table_schema = 'public'
		ORDER BY table_name
	`)
	
	// Test 3: Check key_ranges specifically
	testWithFreshConnection("Check key_ranges table", `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'key_ranges'
		) as key_ranges_exists
	`)
	
	// Test 4: Check found_wallets specifically
	testWithFreshConnection("Check found_wallets table", `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'found_wallets'
		) as found_wallets_exists
	`)

	fmt.Println("\n🎉 All tests completed!")
	fmt.Println("==============================")
}