package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("🔧 Supabase 連線修復工具")
	fmt.Println("========================")
	fmt.Println("")

	// 載入環境變數
	godotenv.Load()

	// 獲取原始 DATABASE_URL
	originalURL := os.Getenv("DATABASE_URL")
	if originalURL == "" {
		fmt.Println("❌ DATABASE_URL 環境變數未設定")
		return
	}

	fmt.Printf("📋 原始 DATABASE_URL: %s...%s\n", originalURL[:30], originalURL[len(originalURL)-20:])
	fmt.Println("")

	// 測試不同的優化配置
	configs := []struct {
		name        string
		modifyURL   func(string) string
		description string
	}{
		{
			name: "原始配置",
			modifyURL: func(url string) string {
				return url
			},
			description: "使用原始 DATABASE_URL",
		},
		{
			name: "添加 SSL require",
			modifyURL: func(url string) string {
				return addOrUpdateParam(url, "sslmode", "require")
			},
			description: "強制使用 SSL 連線",
		},
		{
			name: "SSL prefer + 連線池優化",
			modifyURL: func(url string) string {
				url = addOrUpdateParam(url, "sslmode", "prefer")
				url = addOrUpdateParam(url, "pool_max_conns", "10")
				url = addOrUpdateParam(url, "pool_min_conns", "1")
				return url
			},
			description: "優先使用 SSL，優化連線池",
		},
		{
			name: "超時優化",
			modifyURL: func(url string) string {
				url = addOrUpdateParam(url, "sslmode", "prefer")
				url = addOrUpdateParam(url, "connect_timeout", "30")
				url = addOrUpdateParam(url, "statement_timeout", "30000")
				return url
			},
			description: "增加連線和查詢超時時間",
		},
		{
			name: "Digital Ocean 優化",
			modifyURL: func(url string) string {
				url = addOrUpdateParam(url, "sslmode", "prefer")
				url = addOrUpdateParam(url, "pool_max_conns", "5")
				url = addOrUpdateParam(url, "pool_min_conns", "1")
				url = addOrUpdateParam(url, "connect_timeout", "30")
				url = addOrUpdateParam(url, "pool_max_conn_lifetime", "3600")
				url = addOrUpdateParam(url, "pool_max_conn_idle_time", "1800")
				return url
			},
			description: "針對 Digital Ocean App Platform 的完整優化",
		},
	}

	var bestConfig string
	var bestConfigName string

	for _, config := range configs {
		fmt.Printf("🧪 測試配置: %s\n", config.name)
		fmt.Printf("   描述: %s\n", config.description)

		testURL := config.modifyURL(originalURL)
		success := testConnection(testURL)

		if success {
			fmt.Printf("   ✅ 連線成功！\n")
			if bestConfig == "" {
				bestConfig = testURL
				bestConfigName = config.name
			}
		} else {
			fmt.Printf("   ❌ 連線失敗\n")
		}
		fmt.Println()
	}

	// 輸出建議
	if bestConfig != "" {
		fmt.Println("🎉 找到可用的配置！")
		fmt.Printf("最佳配置: %s\n", bestConfigName)
		fmt.Println("")
		fmt.Println("📋 建議的 DATABASE_URL:")
		fmt.Println(bestConfig)
		fmt.Println("")
		fmt.Println("🔧 在 Digital Ocean App Platform 中更新步驟:")
		fmt.Println("1. 登入 Digital Ocean 控制台")
		fmt.Println("2. 進入 App Platform > 你的應用程式 > Settings")
		fmt.Println("3. 點擊 Environment Variables")
		fmt.Println("4. 編輯 DATABASE_URL 變數")
		fmt.Println("5. 將值更新為上方建議的 URL")
		fmt.Println("6. 點擊 Save 並重新部署應用程式")
	} else {
		fmt.Println("❌ 所有配置都無法連線")
		fmt.Println("")
		fmt.Println("🔍 請檢查:")
		fmt.Println("1. Supabase 專案是否正常運行")
		fmt.Println("2. 資料庫密碼是否正確")
		fmt.Println("3. 網路連線是否正常")
		fmt.Println("4. Supabase 專案是否暫停")
	}
}

// testConnection 測試資料庫連線
func testConnection(databaseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 解析配置
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return false
	}

	// 設定連線池參數
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = time.Minute * 30
	config.HealthCheckPeriod = time.Minute

	// 建立連線池
	pool, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return false
	}
	defer pool.Close()

	// 測試 Ping
	err = pool.Ping(ctx)
	if err != nil {
		return false
	}

	// 測試簡單查詢
	var result int
	err = pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	return err == nil && result == 1
}

// addOrUpdateParam 添加或更新 URL 參數
func addOrUpdateParam(rawURL, key, value string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	query := parsedURL.Query()
	query.Set(key, value)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}