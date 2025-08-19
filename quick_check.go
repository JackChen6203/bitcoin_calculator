package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("⚡ Supabase 快速連線測試")
	fmt.Println("========================")
	fmt.Println("")

	// 載入環境變數
	godotenv.Load()

	// 檢查環境變數
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Println("❌ DATABASE_URL 環境變數未設定")
		fmt.Println("")
		fmt.Println("🔧 解決方法:")
		fmt.Println("1. 在 Digital Ocean App Platform 中設定 DATABASE_URL")
		fmt.Println("2. 或在本地創建 .env 檔案並添加 DATABASE_URL")
		return
	}

	fmt.Println("✅ DATABASE_URL 已設定")
	fmt.Printf("📋 URL 長度: %d 字元\n", len(databaseURL))
	fmt.Println("")

	// 測試基本連線
	fmt.Println("🔗 測試資料庫連線...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		fmt.Printf("❌ 連線失敗: %v\n", err)
		fmt.Println("")
		fmt.Println("🔧 可能的解決方法:")
		fmt.Println("1. 檢查 DATABASE_URL 格式是否正確")
		fmt.Println("2. 確認 Supabase 專案是否正常運行")
		fmt.Println("3. 檢查網路連線")
		fmt.Println("4. 確認資料庫密碼是否正確")
		return
	}
	defer pool.Close()

	fmt.Println("✅ 資料庫連線成功")

	// 測試 Ping
	fmt.Println("🏓 測試 Ping...")
	err = pool.Ping(ctx)
	if err != nil {
		fmt.Printf("❌ Ping 失敗: %v\n", err)
		return
	}
	fmt.Println("✅ Ping 成功")

	// 測試基本查詢
	fmt.Println("📊 測試基本查詢...")
	var result int
	err = pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		fmt.Printf("❌ 查詢失敗: %v\n", err)
		return
	}
	fmt.Printf("✅ 查詢成功，結果: %d\n", result)

	// 測試資料庫版本
	fmt.Println("🔍 檢查資料庫版本...")
	var version string
	err = pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		fmt.Printf("❌ 版本查詢失敗: %v\n", err)
	} else {
		fmt.Printf("✅ 資料庫版本: %s\n", version[:50]+"...")
	}

	// 檢查必要資料表
	fmt.Println("📋 檢查必要資料表...")
	requiredTables := []string{"key_ranges", "found_wallets"}
	allTablesExist := true

	for _, table := range requiredTables {
		var exists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", table).Scan(&exists)
		if err != nil {
			fmt.Printf("❌ 檢查資料表 %s 失敗: %v\n", table, err)
			allTablesExist = false
		} else if !exists {
			fmt.Printf("❌ 資料表 %s 不存在\n", table)
			allTablesExist = false
		} else {
			fmt.Printf("✅ 資料表 %s 存在\n", table)
		}
	}

	// 測試資料表內容
	if allTablesExist {
		fmt.Println("📊 檢查資料表內容...")

		// 檢查 key_ranges
		var keyRangeCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM key_ranges").Scan(&keyRangeCount)
		if err != nil {
			fmt.Printf("❌ 查詢 key_ranges 失敗: %v\n", err)
		} else {
			fmt.Printf("✅ key_ranges 資料表有 %d 筆記錄\n", keyRangeCount)
		}

		// 檢查 found_wallets
		var foundWalletCount int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM found_wallets").Scan(&foundWalletCount)
		if err != nil {
			fmt.Printf("❌ 查詢 found_wallets 失敗: %v\n", err)
		} else {
			fmt.Printf("✅ found_wallets 資料表有 %d 筆記錄\n", foundWalletCount)
		}
	}

	// 測試連線池狀態
	fmt.Println("🏊 檢查連線池狀態...")
	stats := pool.Stat()
	fmt.Printf("✅ 連線池狀態:\n")
	fmt.Printf("   - 總連線數: %d\n", stats.TotalConns())
	fmt.Printf("   - 閒置連線數: %d\n", stats.IdleConns())
	fmt.Printf("   - 使用中連線數: %d\n", stats.AcquiredConns())

	fmt.Println("")
	fmt.Println("🎉 所有測試完成！")
	fmt.Println("")

	if !allTablesExist {
		fmt.Println("⚠️ 部分資料表不存在，請執行:")
		fmt.Println("   go run setup_database.go")
		fmt.Println("   或手動執行 create_tables.sql")
	} else {
		fmt.Println("✅ Supabase 連線完全正常，可以開始使用！")
	}

	fmt.Println("")
	fmt.Println("📋 如果在 Digital Ocean App Platform 上運行:")
	fmt.Println("1. 確保此測試通過")
	fmt.Println("2. 檢查應用程式日誌是否有其他錯誤")
	fmt.Println("3. 確認應用程式的健康檢查端點正常")
}
