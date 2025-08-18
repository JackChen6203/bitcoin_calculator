package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

const (
	// Number of concurrent workers to check balances
	concurrency = 200
	// A unique ID for this worker instance
	workerID = "worker-"
)

// KeyRange represents a work unit from the database
type KeyRange struct {
	ID          int64
	StartKeyHex string
	EndKeyHex   string
}

// Config holds application configuration
type Config struct {
	dbpool       *pgxpool.Pool
	workerID     string
	discordWebhook string
}

// DiscordWebhookPayload represents the payload structure for Discord webhook
type DiscordWebhookPayload struct {
	Content string `json:"content"`
	Embeds  []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents an embed in Discord message
type DiscordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp"`
}

// DiscordEmbedField represents a field in Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

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

	// Get Discord webhook URL from environment (optional)
	discordWebhook := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordWebhook == "" {
		log.Println("DISCORD_WEBHOOK_URL not set, Discord notifications disabled")
	} else {
		log.Println("Discord notifications enabled")
	}

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup database connection pool
	dbpool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer dbpool.Close()

	// Generate a unique ID for this worker
	hostname, _ := os.Hostname()
	appConfig := &Config{
		dbpool:        dbpool,
		workerID:      fmt.Sprintf("%s%s-%d", workerID, hostname, time.Now().UnixNano()),
		discordWebhook: discordWebhook,
	}
	log.Printf("Starting worker: %s", appConfig.workerID)

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutdown signal received. Finishing current job...")
		cancel()
	}()

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Worker shutting down.")
			return
		default:
			keyRange, err := claimWorkUnit(ctx, appConfig)
			if err != nil {
				log.Printf("Could not claim a work unit: %v. Waiting...", err)
				time.Sleep(10 * time.Second)
				continue
			}

			if keyRange == nil {
				log.Println("No pending work units found. All work is done. Exiting.")
				return
			}

			log.Printf("Claimed work unit #%d. Processing range: %s to %s", keyRange.ID, keyRange.StartKeyHex, keyRange.EndKeyHex)
			processKeyRange(ctx, appConfig, keyRange)

			err = markWorkUnitComplete(ctx, appConfig, keyRange.ID)
			if err != nil {
				log.Printf("Failed to mark work unit #%d as complete: %v", keyRange.ID, err)
			} else {
				log.Printf("Successfully completed work unit #%d.", keyRange.ID)
			}
		}
	}
}

// Atomically claims a work unit from the database
func claimWorkUnit(ctx context.Context, config *Config) (*KeyRange, error) {
	query := `
        UPDATE key_ranges
        SET status = 'processing', worker_id = $1, claimed_at = NOW()
        WHERE id = (
            SELECT id
            FROM key_ranges
            WHERE status = 'pending'
            ORDER BY id
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        RETURNING id, start_key_hex, end_key_hex;
    `
	row := config.dbpool.QueryRow(ctx, query, config.workerID)
	kr := &KeyRange{}
	err := row.Scan(&kr.ID, &kr.StartKeyHex, &kr.EndKeyHex)
	if err != nil {
		// pgx.ErrNoRows is expected when no work is available
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan claimed work unit: %w", err)
	}
	return kr, nil
}

// Processes all private keys within a given range
func processKeyRange(ctx context.Context, config *Config, kr *KeyRange) {
	startKey := new(big.Int)
	startKey.SetString(kr.StartKeyHex, 16)

	endKey := new(big.Int)
	endKey.SetString(kr.EndKeyHex, 16)

	var wg sync.WaitGroup
	keyChan := make(chan *big.Int, concurrency)

	// Create a pool of workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pKeyInt := range keyChan {
				select {
				case <-ctx.Done():
					return // Exit if context is cancelled
				default:
					address, privKey, err := checkPrivateKey(pKeyInt)
					if err != nil {
						continue // Skip if address generation fails
					}

					balance, err := getBalance(ctx, address)
					if err != nil {
						// Log this error if needed, but continue processing
						continue
					}

					if balance > 0 {
						log.Printf("!!! SUCCESS: Found wallet with balance! Address: %s, Balance: %d satoshis", address, balance)
						saveFoundWallet(ctx, config, privKey, address, balance)
					}
				}
			}
		}()
	}

	// Feed the workers with keys
	currentKey := new(big.Int).Set(startKey)
	one := big.NewInt(1)
	for currentKey.Cmp(endKey) <= 0 {
		select {
		case <-ctx.Done():
			log.Println("Stopping key generation due to shutdown signal.")
			close(keyChan)
			wg.Wait()
			return
		default:
			// Must send a copy, not the pointer itself
			keyToSend := new(big.Int).Set(currentKey)
			keyChan <- keyToSend
			currentKey.Add(currentKey, one)
		}
	}

	close(keyChan)
	wg.Wait()
}


// Checks a single private key and returns the corresponding compressed address and private key object.
func checkPrivateKey(pKeyInt *big.Int) (string, *btcec.PrivateKey, error) {
	// Ensure the private key is within the valid range
	if pKeyInt.Cmp(big.NewInt(0)) <= 0 || pKeyInt.Cmp(btcec.S256().N) >= 0 {
		return "", nil, fmt.Errorf("private key is out of range")
	}

	// Convert big.Int to byte slice for btcec.PrivKeyFromBytes
	privKeyBytes := pKeyInt.Bytes()
	// Pad to 32 bytes if necessary
	paddedKey := make([]byte, 32)
	copy(paddedKey[32-len(privKeyBytes):], privKeyBytes)

	// Create a btcec.PrivateKey from the byte slice
	privKey, _ := btcec.PrivKeyFromBytes(paddedKey)

	// Get the public key (X, Y coordinates)
	pubKey := privKey.PubKey()

	// Manually create compressed public key bytes
	var compressedPubKeyBytes []byte
	if pubKey.Y().Bit(0) == 0 { // Y is even
		compressedPubKeyBytes = make([]byte, 33)
		compressedPubKeyBytes[0] = 0x02
	} else { // Y is odd
		compressedPubKeyBytes = make([]byte, 33)
		compressedPubKeyBytes[0] = 0x03
	}
	// Ensure X coordinate is 32 bytes long
	xBytes := pubKey.X().Bytes()
	copy(compressedPubKeyBytes[1+32-len(xBytes):], xBytes)

	// Use btcutil.NewAddressPubKey to create the address from the compressed public key
	addr, err := btcutil.NewAddressPubKey(compressedPubKeyBytes, &chaincfg.MainNetParams)
	if err != nil {
		return "", nil, err
	}

	// Encode the address to its string representation
	address := addr.EncodeAddress()

	return address, privKey, nil
}


// getBalance checks the balance of a Bitcoin address using a public API
func getBalance(ctx context.Context, address string) (int64, error) {
	// Using blockchain.info API
	url := fmt.Sprintf("https://blockchain.info/q/addressbalance/%s", address)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	// Add a timeout to the client to avoid hanging
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var balance int64
	// The API returns a simple number
	_, err = fmt.Sscan(string(body), &balance)
	if err != nil {
		return 0, fmt.Errorf("failed to parse balance from API response: %w", err)
	}

	return balance, nil
}


// saveFoundWallet saves the details of a wallet with a balance to the database
func saveFoundWallet(ctx context.Context, config *Config, privKey *btcec.PrivateKey, address string, balance int64) {
	wif, err := btcutil.NewWIF(privKey, &chaincfg.MainNetParams, true)
	if err != nil {
		log.Printf("Failed to generate WIF for private key: %v", err)
		return
	}

	query := `
        INSERT INTO found_wallets (private_key_wif, address, balance_satoshi, worker_id)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (private_key_wif) DO NOTHING;
    `
	_, err = config.dbpool.Exec(ctx, query, wif.String(), address, balance, config.workerID)
	if err != nil {
		log.Printf("Failed to save found wallet to database: %v", err)
	} else {
		log.Printf("Successfully saved wallet %s to database.", address)
		// Send Discord notification if webhook is configured
		if config.discordWebhook != "" {
			go sendDiscordNotification(ctx, config.discordWebhook, address, balance, wif.String())
		}
	}
}

// markWorkUnitComplete marks a work unit as 'completed' in the database
func markWorkUnitComplete(ctx context.Context, config *Config, rangeID int64) error {
	query := `
        UPDATE key_ranges
        SET status = 'completed', completed_at = NOW()
        WHERE id = $1;
    `
	_, err := config.dbpool.Exec(ctx, query, rangeID)
	return err
}

// sendDiscordNotification sends a notification to Discord webhook when a wallet with balance is found
func sendDiscordNotification(ctx context.Context, webhookURL, address string, balance int64, privateKey string) {
	// Convert satoshis to BTC for better readability
	btcAmount := float64(balance) / 100000000.0

	// Create embed with wallet information
	embed := DiscordEmbed{
		Title:       "🎯 發現有餘額的比特幣錢包！",
		Description: "自動掃描系統發現了一個有餘額的比特幣地址",
		Color:       0x00FF00, // Green color
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []DiscordEmbedField{
			{
				Name:   "💰 地址",
				Value:  fmt.Sprintf("`%s`", address),
				Inline: false,
			},
			{
				Name:   "💎 餘額",
				Value:  fmt.Sprintf("**%.8f BTC** (%d satoshis)", btcAmount, balance),
				Inline: true,
			},
			{
				Name:   "🔑 私鑰 (WIF)",
				Value:  fmt.Sprintf("||​`%s`​||", privateKey), // Spoiler tags for security
				Inline: false,
			},
			{
				Name:   "⏰ 發現時間",
				Value:  fmt.Sprintf("<t:%d:F>", time.Now().Unix()),
				Inline: true,
			},
		},
	}

	// Create payload
	payload := DiscordWebhookPayload{
		Content: fmt.Sprintf("🚨 **發現有餘額的錢包！** 🚨\n地址: %s\n餘額: %.8f BTC\n私鑰: ||%s||", address, btcAmount, privateKey),
		Embeds:  []DiscordEmbed{embed},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal Discord payload: %v", err)
		return
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to create Discord webhook request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request with timeout
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send Discord notification: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("✅ Discord notification sent successfully for wallet: %s", address)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("❌ Discord notification failed. Status: %d, Response: %s", resp.StatusCode, string(body))
	}
}
