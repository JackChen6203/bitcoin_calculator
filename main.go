//go:build !populate
// +build !populate

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

	// Initialize app config without database first
	hostname, _ := os.Hostname()
	appConfig := &Config{
		dbpool:        nil, // Will be set later if database connection succeeds
		workerID:      fmt.Sprintf("%s%s-%d", workerID, hostname, time.Now().UnixNano()),
		discordWebhook: discordWebhook,
	}

	// Start HTTP health check server immediately (even without database)
	go startHealthServer(ctx, appConfig)
	log.Printf("Health server started on port %s", getPort())

	// Try to connect to database
	if databaseURL == "" {
		log.Println("⚠️  DATABASE_URL environment variable is not set")
		log.Println("⚠️  Running in health-check-only mode")
		log.Println("⚠️  Bitcoin scanning is disabled until database is configured")
		
		// Keep the app running for health checks
		select {
		case <-ctx.Done():
			log.Println("Application shutting down...")
			return
		}
	}

	// Setup database connection pool
	log.Println("Connecting to database...")
	dbpool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		log.Printf("❌ Unable to connect to database: %v", err)
		log.Println("⚠️  Running in health-check-only mode")
		log.Println("⚠️  Bitcoin scanning is disabled until database connection is fixed")
		
		// Keep the app running for health checks
		select {
		case <-ctx.Done():
			log.Println("Application shutting down...")
			return
		}
	}
	defer dbpool.Close()

	// Update config with database connection
	appConfig.dbpool = dbpool
	log.Println("✅ Database connection established")
	
	// Add detailed database diagnostics
	performDatabaseDiagnostics(ctx, dbpool)
	
	log.Printf("🚀 Starting Bitcoin scanner worker: %s", appConfig.workerID)

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

// getPort returns the port to use for the HTTP server
func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port for Digital Ocean
	}
	return port
}

// startHealthServer starts a simple HTTP server for health checks
// Digital Ocean App Platform requires HTTP endpoints for health checks
func startHealthServer(ctx context.Context, config *Config) {
	port := getPort()

	mux := http.NewServeMux()
	
	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check if database connection exists and is healthy
		if config.dbpool == nil {
			// No database connection - return partial health status
			w.WriteHeader(http.StatusOK) // Still return 200 for Digital Ocean health checks
			fmt.Fprintf(w, `{"status":"partial","message":"HTTP server healthy, database not configured","worker_id":"%s","timestamp":"%s"}`, 
				config.workerID, time.Now().Format(time.RFC3339))
			return
		}
		
		// Check database connection
		err := config.dbpool.Ping(ctx)
		if err != nil {
			w.WriteHeader(http.StatusOK) // Still return 200 for Digital Ocean health checks
			fmt.Fprintf(w, `{"status":"partial","message":"HTTP server healthy, database connection failed","error":"%s","worker_id":"%s","timestamp":"%s"}`, 
				err.Error(), config.workerID, time.Now().Format(time.RFC3339))
			return
		}
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy","message":"All systems operational","worker_id":"%s","timestamp":"%s"}`, 
			config.workerID, time.Now().Format(time.RFC3339))
	})

	// Status endpoint for monitoring
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		dbStatus := "not configured"
		if config.dbpool != nil {
			if err := config.dbpool.Ping(ctx); err == nil {
				dbStatus = "connected"
			} else {
				dbStatus = "connection failed"
			}
		}
		
		fmt.Fprintf(w, `{
		"service": "Bitcoin Private Key Scanner",
		"worker_id": "%s",
		"status": "running",
		"database_status": "%s",
		"discord_enabled": %t,
		"scanning_active": %t,
		"timestamp": "%s"
	}`, config.workerID, dbStatus, config.discordWebhook != "", config.dbpool != nil, time.Now().Format(time.RFC3339))
	})

	// Root endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		
		dbStatus := "❌ Not configured"
		scanningStatus := "❌ Inactive"
		
		if config.dbpool != nil {
			if err := config.dbpool.Ping(ctx); err == nil {
				dbStatus = "✅ Connected"
				scanningStatus = "✅ Active"
			} else {
				dbStatus = "⚠️ Connection failed"
			}
		}
		
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>Bitcoin Scanner</title>
	<style>
		body{font-family:Arial,sans-serif;margin:40px;background:#f5f5f5;}
		.container{background:white;padding:20px;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,0.1);}
		.status{margin:10px 0;}
		.healthy{color:#28a745;}
		.warning{color:#ffc107;}
		.error{color:#dc3545;}
	</style>
</head>
<body>
	<div class="container">
		<h1>🎯 Bitcoin Private Key Scanner</h1>
		<div class="status"><strong>HTTP Server:</strong> <span class="healthy">✅ Running</span></div>
		<div class="status"><strong>Database:</strong> %s</div>
		<div class="status"><strong>Bitcoin Scanning:</strong> %s</div>
		<div class="status"><strong>Discord Notifications:</strong> %s</div>
		<div class="status"><strong>Worker ID:</strong> %s</div>
		<div class="status"><strong>Timestamp:</strong> %s</div>
		<hr>
		<h3>API Endpoints:</h3>
		<ul>
			<li><a href="/health">/health</a> - Health check (JSON)</li>
			<li><a href="/status">/status</a> - Detailed status (JSON)</li>
		</ul>
		<hr>
		<p><small>To configure the database, set the DATABASE_URL environment variable.</small></p>
	</div>
</body>
</html>`, dbStatus, scanningStatus,
			map[bool]string{true: "✅ Enabled", false: "❌ Disabled"}[config.discordWebhook != ""],
			config.workerID, time.Now().Format(time.RFC3339))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Starting health server on port %s", port)
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	// Graceful shutdown when context is cancelled
	<-ctx.Done()
	log.Println("Shutting down health server...")
	
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Health server shutdown error: %v", err)
	}
}

// performDatabaseDiagnostics performs detailed database diagnostics
func performDatabaseDiagnostics(ctx context.Context, dbpool *pgxpool.Pool) {
	log.Println("🔍 Starting database diagnostics...")
	
	// Test 1: Basic connection test
	log.Println("🔍 Test 1: Basic connection ping")
	err := dbpool.Ping(ctx)
	if err != nil {
		log.Printf("❌ Ping failed: %v", err)
		return
	}
	log.Println("✅ Ping successful")
	
	// Test 2: Get database version and basic info
	log.Println("🔍 Test 2: Database version and info")
	var version string
	err = dbpool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Printf("❌ Failed to get version: %v", err)
	} else {
		log.Printf("✅ Database version: %s", version[:100] + "...") // Truncate for readability
	}
	
	// Test 3: Check current database and schema
	log.Println("🔍 Test 3: Current database and schema")
	var currentDB, currentSchema, currentUser string
	err = dbpool.QueryRow(ctx, "SELECT current_database(), current_schema(), current_user").Scan(&currentDB, &currentSchema, &currentUser)
	if err != nil {
		log.Printf("❌ Failed to get database info: %v", err)
	} else {
		log.Printf("✅ Current database: %s", currentDB)
		log.Printf("✅ Current schema: %s", currentSchema)
		log.Printf("✅ Current user: %s", currentUser)
	}
	
	// Test 4: List all tables in current schema
	log.Println("🔍 Test 4: Listing all tables in current schema")
	rows, err := dbpool.Query(ctx, `
		SELECT table_name, table_type 
		FROM information_schema.tables 
		WHERE table_schema = current_schema() 
		ORDER BY table_name
	`)
	if err != nil {
		log.Printf("❌ Failed to list tables: %v", err)
	} else {
		var tableCount int
		log.Println("✅ Tables in current schema:")
		for rows.Next() {
			var tableName, tableType string
			err = rows.Scan(&tableName, &tableType)
			if err != nil {
				log.Printf("  ❌ Error scanning table: %v", err)
				continue
			}
			log.Printf("  - %s (%s)", tableName, tableType)
			tableCount++
		}
		rows.Close()
		log.Printf("✅ Total tables found: %d", tableCount)
	}
	
	// Test 5: Check if key_ranges table exists specifically
	log.Println("🔍 Test 5: Checking for key_ranges table specifically")
	var exists bool
	err = dbpool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = current_schema() 
			AND table_name = 'key_ranges'
		)
	`).Scan(&exists)
	if err != nil {
		log.Printf("❌ Failed to check key_ranges existence: %v", err)
	} else if exists {
		log.Println("✅ key_ranges table EXISTS")
		
		// Test 5a: Check table structure
		log.Println("🔍 Test 5a: key_ranges table structure")
		colRows, err := dbpool.Query(ctx, `
			SELECT column_name, data_type, is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = current_schema() 
			AND table_name = 'key_ranges'
			ORDER BY ordinal_position
		`)
		if err != nil {
			log.Printf("❌ Failed to get column info: %v", err)
		} else {
			log.Println("✅ key_ranges columns:")
			for colRows.Next() {
				var colName, dataType, nullable, defaultVal string
				err = colRows.Scan(&colName, &dataType, &nullable, &defaultVal)
				if err != nil {
					log.Printf("  ❌ Error scanning column: %v", err)
					continue
				}
				log.Printf("  - %s: %s (nullable: %s, default: %s)", colName, dataType, nullable, defaultVal)
			}
			colRows.Close()
		}
		
		// Test 5b: Count rows in key_ranges
		var rowCount int
		err = dbpool.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&rowCount)
		if err != nil {
			log.Printf("❌ Failed to count key_ranges rows: %v", err)
		} else {
			log.Printf("✅ key_ranges contains %d rows", rowCount)
			
			// Test 5c: Show status distribution
			if rowCount > 0 {
				statusRows, err := dbpool.Query(ctx, "SELECT status, COUNT(*) FROM key_ranges GROUP BY status ORDER BY status")
				if err != nil {
					log.Printf("❌ Failed to get status distribution: %v", err)
				} else {
					log.Println("✅ Status distribution:")
					for statusRows.Next() {
						var status string
						var count int
						err = statusRows.Scan(&status, &count)
						if err != nil {
							log.Printf("  ❌ Error scanning status: %v", err)
							continue
						}
						log.Printf("  - %s: %d", status, count)
					}
					statusRows.Close()
				}
			}
		}
	} else {
		log.Println("❌ key_ranges table does NOT exist")
	}
	
	// Test 6: Check if found_wallets table exists
	log.Println("🔍 Test 6: Checking for found_wallets table")
	err = dbpool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = current_schema() 
			AND table_name = 'found_wallets'
		)
	`).Scan(&exists)
	if err != nil {
		log.Printf("❌ Failed to check found_wallets existence: %v", err)
	} else if exists {
		log.Println("✅ found_wallets table EXISTS")
	} else {
		log.Println("❌ found_wallets table does NOT exist")
	}
	
	// Test 7: Test the exact query that's failing
	log.Println("🔍 Test 7: Testing the exact claimWorkUnit query")
	testQuery := `
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
	_, err = dbpool.Query(ctx, testQuery, "diagnostic-test-worker")
	if err != nil {
		log.Printf("❌ Exact claimWorkUnit query failed: %v", err)
		log.Printf("❌ This is the same error the application is experiencing")
	} else {
		log.Println("✅ claimWorkUnit query executed successfully (no work units available or claimed)")
	}
	
	log.Println("🔍 Database diagnostics completed")
	log.Println("========================================")
}
