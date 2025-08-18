# Project Architecture: Bitcoin Private Key Scanner

This project implements a distributed system for scanning Bitcoin private key ranges to identify wallets with non-zero balances. It follows a client-worker pattern, leveraging a PostgreSQL database for work distribution and persistence.

## Components:

1.  **PostgreSQL Database:**
    *   **`key_ranges` table:** Stores defined ranges of Bitcoin private keys. Each range is a "work unit" with a `status` (e.g., 'pending', 'processing', 'completed') and `worker_id` to track which worker is processing it.
    *   **`found_wallets` table:** Stores details of Bitcoin wallets found to have a balance, including the private key (WIF), address, and balance in satoshis.

2.  **Worker Application (`main.go`):**
    *   **Purpose:** The core component responsible for processing private key ranges.
    *   **Work Claiming:** Atomically claims a 'pending' work unit from the `key_ranges` table, marking it as 'processing'.
    *   **Key Generation & Address Derivation:** Iterates through the private keys within the claimed range, generating corresponding Bitcoin addresses (compressed P2PKH).
    *   **Balance Checking:** Queries an external API (currently `blockchain.info`) to check the balance of each derived Bitcoin address.
    *   **Wallet Saving:** If a wallet is found with a non-zero balance, its details (private key WIF, address, balance) are saved to the `found_wallets` table.
    *   **Concurrency:** Utilizes Go goroutines and channels to process multiple private keys concurrently within a single worker instance.
    *   **Graceful Shutdown:** Implements signal handling for graceful termination, allowing the current work unit to finish before exiting.
    *   **Work Completion:** Marks the processed work unit as 'completed' in the database.

3.  **Range Population Utility (`populate_ranges.go.temp`):**
    *   **Purpose:** A standalone utility script designed to pre-populate the `key_ranges` table in the database.
    *   **Functionality:** Generates a specified number of private key ranges (work units) and inserts them into the `key_ranges` table with a 'pending' status, making them available for workers to claim.
    *   **Usage:** Intended to be run once (or periodically) to set up the initial work queue for the system.

## Data Flow:

1.  The `populate_ranges.go.temp` utility is executed to fill the `key_ranges` table with pending work units.
2.  One or more instances of the `main.go` worker application are started.
3.  Each worker continuously attempts to claim a 'pending' work unit from the database.
4.  Upon claiming a unit, the worker processes the private keys within its assigned range.
5.  For each private key, a Bitcoin address is derived, and its balance is checked via the `blockchain.info` API.
6.  If a balance is found, the wallet details are stored in the `found_wallets` table.
7.  Once a range is fully processed, the worker marks the corresponding entry in `key_ranges` as 'completed'.
8.  Workers continue this cycle until no more 'pending' work units are available.

## Dependencies:

*   **Go:** Programming language.
*   **`github.com/btcsuite/btcd`:** Bitcoin protocol implementation for key and address handling.
*   **`github.com/jackc/pgx/v4`:** PostgreSQL driver for Go.
*   **`github.com/joho/godotenv`:** For loading environment variables from a `.env` file.
*   **`blockchain.info` API:** External service used for checking Bitcoin address balances.
