package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/joho/godotenv"
	"os"
)

func main() {
	fmt.Println("🔧 Populating Test Key Ranges")
	fmt.Println("=============================")

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Get database URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

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

	// Create a few test ranges (small ranges for testing)
	fmt.Println("🔍 Creating test key ranges...")
	
	// Start from a small number for testing (around 2^20)
	startKey := big.NewInt(1048576) // 2^20
	rangeSize := big.NewInt(1000)   // Small ranges of 1000 keys each

	ranges := []struct {
		start *big.Int
		end   *big.Int
	}{
		{new(big.Int).Set(startKey), new(big.Int).Add(startKey, rangeSize)},
		{new(big.Int).Add(startKey, rangeSize), new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(2)))},
		{new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(2))), new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(3)))},
		{new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(3))), new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(4)))},
		{new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(4))), new(big.Int).Add(startKey, new(big.Int).Mul(rangeSize, big.NewInt(5)))},
	}

	// Clear existing ranges first
	_, err = conn.Exec(ctx, "DELETE FROM key_ranges")
	if err != nil {
		log.Printf("⚠️ Failed to clear existing ranges: %v", err)
	} else {
		fmt.Println("✅ Cleared existing ranges")
	}

	// Insert ranges
	for i, r := range ranges {
		startHex := fmt.Sprintf("%x", r.start)
		endHex := fmt.Sprintf("%x", r.end)
		
		_, err = conn.Exec(ctx, `
			INSERT INTO key_ranges (start_key_hex, end_key_hex, status) 
			VALUES ($1, $2, 'pending')
		`, startHex, endHex)
		
		if err != nil {
			log.Printf("❌ Failed to insert range %d: %v", i+1, err)
		} else {
			fmt.Printf("✅ Inserted range %d: %s to %s\n", i+1, startHex, endHex)
		}
	}

	// Count total ranges
	var count int
	err = conn.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&count)
	if err != nil {
		log.Printf("❌ Failed to count ranges: %v", err)
	} else {
		fmt.Printf("📊 Total key ranges: %d\n", count)
	}

	// Show status distribution
	rows, err := conn.Query(ctx, "SELECT status, COUNT(*) FROM key_ranges GROUP BY status ORDER BY status")
	if err != nil {
		log.Printf("❌ Failed to get status distribution: %v", err)
	} else {
		fmt.Println("📊 Status distribution:")
		for rows.Next() {
			var status string
			var statusCount int
			err = rows.Scan(&status, &statusCount)
			if err != nil {
				log.Printf("❌ Error scanning status: %v", err)
				continue
			}
			fmt.Printf("  - %s: %d\n", status, statusCount)
		}
		rows.Close()
	}

	fmt.Println("\n🎉 Test ranges populated successfully!")
	fmt.Println("Your Bitcoin scanner can now start processing!")
	fmt.Println("=========================================")
}