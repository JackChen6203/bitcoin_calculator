# Project Understanding: Bitcoin Private Key Scanner

This project is a Go-based distributed system designed to scan ranges of Bitcoin private keys to discover wallets with existing balances. It operates on a producer-consumer model, where a separate utility populates work units (key ranges) into a PostgreSQL database, and multiple worker instances then claim and process these units.

## Core Functionality:

1.  **Work Distribution:** The system uses a `key_ranges` table in PostgreSQL to manage work units. Each unit represents a hexadecimal range of Bitcoin private keys. Workers claim these units, process them, and mark them as complete.
2.  **Private Key to Address Derivation:** For each private key within a claimed range, the worker derives the corresponding compressed Bitcoin address (P2PKH).
3.  **Balance Checking:** The derived Bitcoin addresses are then queried against the `blockchain.info` API to check for any existing balance.
4.  **Wallet Persistence:** If a non-zero balance is found for an address, the private key (in WIF format), the address, and its balance are stored in a `found_wallets` table in the PostgreSQL database.
5.  **Concurrency:** The `main.go` worker leverages Go's concurrency features (goroutines and channels) to efficiently check multiple private keys in parallel.

## Key Components and Their Roles:

*   **`main.go` (Worker):** This is the primary application that performs the actual scanning. It connects to the PostgreSQL database, claims work units, iterates through private keys, derives addresses, checks balances via an external API, and saves any found wallets. It also includes graceful shutdown handling.
*   **`populate_ranges.go.temp` (Utility):** This is a temporary utility script used to pre-fill the `key_ranges` table with initial work units. It defines a starting key, a range size, and the number of ranges to create, effectively setting up the scanning tasks.
*   **`go.mod` and `go.sum`:** These files manage the Go module dependencies, including `btcsuite/btcd` for Bitcoin cryptography, `jackc/pgx/v4` for PostgreSQL interaction, and `joho/godotenv` for environment variable loading.
*   **`.env`:** This file is used to configure environment variables, primarily the `DATABASE_URL` for connecting to the PostgreSQL database.
*   **`main_test.go`:** Contains unit tests for core functionalities like Bitcoin address generation from a private key and mocking the `getBalance` function's API calls for testing purposes.

## Workflow:

1.  **Setup:** A PostgreSQL database needs to be running with the `key_ranges` and `found_wallets` tables created (schema not provided in the current files, but implied).
2.  **Population:** The `populate_ranges.go.temp` script is executed once to seed the `key_ranges` table with pending work.
3.  **Execution:** One or more instances of the `main.go` worker are started. Each worker will continuously:
    *   Claim a pending key range from the database.
    *   Generate and check Bitcoin addresses within that range concurrently.
    *   Save any found wallets.
    *   Mark the range as completed.
4.  **Monitoring:** The `found_wallets` table can be monitored for any discovered Bitcoin wallets.

## Potential Improvements/Considerations:

*   **Error Handling:** While some error handling is present, robust error handling for API calls (e.g., rate limiting, retries, different API endpoints) could be improved.
*   **Database Schema:** The database schema for `key_ranges` and `found_wallets` is not explicitly defined in the provided files, which would be crucial for setup.
*   **API Reliability:** Relying on a single public API (`blockchain.info`) for balance checks might introduce a single point of failure or rate limiting issues. Using multiple APIs or a local Bitcoin node could enhance reliability.
*   **Key Range Management:** The `populate_ranges.go.temp` is a temporary script. A more robust system might involve dynamic range generation or a more sophisticated work management system.
*   **Security:** Storing private keys, even in WIF format, requires careful consideration of security best practices for the database and application environment.
