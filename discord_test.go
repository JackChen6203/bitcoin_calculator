package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/joho/godotenv"
)

// testDiscordNotification tests the Discord webhook functionality
func testDiscordNotification() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Get Discord webhook URL
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("DISCORD_WEBHOOK_URL environment variable is not set")
	}

	// Create a test private key (this is just for testing, not a real wallet)
	testPrivKey, _ := btcec.NewPrivateKey()
	testWIF := "test_private_key_wif_format"
	testAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" // Satoshi's genesis block address
	testBalance := int64(5000000000)                       // 50 BTC in satoshis

	log.Printf("Testing Discord notification...")
	log.Printf("Webhook URL: %s", webhookURL[:50]+"...")
	log.Printf("Test Address: %s", testAddress)
	log.Printf("Test Balance: %d satoshis", testBalance)

	// Send test notification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sendDiscordNotification(ctx, webhookURL, testAddress, testBalance, testWIF)

	log.Printf("Test completed. Check your Discord channel for the notification.")
}

func main() {
	testDiscordNotification()
}