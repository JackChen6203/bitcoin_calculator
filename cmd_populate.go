//go:build populate
// +build populate

package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
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

	// Setup database connection pool
	dbpool, err := pgxpool.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbpool.Close()

	// The valid range for Bitcoin private keys is [1, N-1] where N is the curve order.
	// N = FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
	maxKey := new(big.Int)
	maxKey.SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364140", 16)

	// Let's start from a known range with some old wallets for demonstration.
	// This is a very small number for a private key, but good for testing.
	startKey := new(big.Int)
	startKey.SetString("10000000000000000", 16) // Starting from a higher number

	// Define the size of each work unit (range)
	rangeSize := big.NewInt(1000000) // 1 million keys per range

	// How many ranges to create
	numRanges := 1000 // Create 1000 work units

	log.Printf("Populating `key_ranges` table with %d work units...", numRanges)

	currentStartKey := new(big.Int).Set(startKey)
	one := big.NewInt(1)

	for i := 0; i < numRanges; i++ {
		currentEndKey := new(big.Int).Add(currentStartKey, rangeSize)
		currentEndKey.Sub(currentEndKey, one) // end key is inclusive

		if currentEndKey.Cmp(maxKey) > 0 {
			currentEndKey.Set(maxKey)
		}

		startHex := fmt.Sprintf("%x", currentStartKey)
		endHex := fmt.Sprintf("%x", currentEndKey)

		query := `
            INSERT INTO key_ranges (start_key_hex, end_key_hex, status)
            VALUES ($1, $2, 'pending')
            ON CONFLICT (start_key_hex, end_key_hex) DO NOTHING;
        `
		_, err := dbpool.Exec(context.Background(), query, startHex, endHex)
		if err != nil {
			log.Printf("Failed to insert range %s - %s: %v", startHex, endHex, err)
		} else {
			fmt.Printf(".")
		}

		if currentEndKey.Cmp(maxKey) == 0 {
			log.Println("\nReached the maximum possible key. Stopping.")
		}

		currentStartKey.Add(currentEndKey, one)
	}

	log.Println("\nFinished populating key ranges.")
}