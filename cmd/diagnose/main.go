package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/joho/godotenv"
)

// DiagnosticResult 診斷結果結構
type DiagnosticResult struct {
	Test       string
	Status     string // "PASS", "FAIL", "WARN"
	Message    string
	Suggestion string
	Details    map[string]interface{}
}

// ConnectionDiagnostic 連線診斷器
type ConnectionDiagnostic struct {
	Results []DiagnosticResult
}

func main() {
	fmt.Println("🔍 Supabase 連線診斷工具")
	fmt.Println("==========================")
	fmt.Println("專為 Digital Ocean App Platform 設計")
	fmt.Println("")

	diag := &ConnectionDiagnostic{}

	// 載入環境變數
	diag.loadEnvironment()

	// 執行所有診斷測試
	diag.checkEnvironmentVariables()
	diag.checkDatabaseURL()
	diag.testNetworkConnectivity()
	diag.testSSLModes()
	diag.testConnectionPools()
	diag.testDatabaseOperations()
	diag.checkSupabaseSpecific()

	// 輸出診斷報告
	diag.printReport()

	// 提供修復建議
	diag.provideFixes()
}

// loadEnvironment 載入環境變數
func (d *ConnectionDiagnostic) loadEnvironment() {
	// 嘗試載入 .env 檔案（本地開發用）
	err := godotenv.Load()
	if err != nil {
		d.addResult("環境變數載入", "WARN", "未找到 .env 檔案，使用系統環境變數", "在生產環境中這是正常的", nil)
	} else {
		d.addResult("環境變數載入", "PASS", "成功載入 .env 檔案", "", nil)
	}
}

// checkEnvironmentVariables 檢查環境變數
func (d *ConnectionDiagnostic) checkEnvironmentVariables() {
	fmt.Println("📋 檢查環境變數...")

	// 檢查 DATABASE_URL
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		d.addResult("DATABASE_URL", "FAIL", "DATABASE_URL 環境變數未設定", "在 Digital Ocean App Platform 中設定 DATABASE_URL 環境變數", nil)
		return
	}

	// 檢查 URL 格式
	if !strings.HasPrefix(databaseURL, "postgresql://") && !strings.HasPrefix(databaseURL, "postgres://") {
		d.addResult("DATABASE_URL 格式", "FAIL", "DATABASE_URL 格式不正確", "確保 URL 以 postgresql:// 或 postgres:// 開頭", map[string]interface{}{
			"current_url_prefix": databaseURL[:20],
		})
		return
	}

	d.addResult("DATABASE_URL", "PASS", "DATABASE_URL 已設定", "", map[string]interface{}{
		"url_length": len(databaseURL),
		"url_prefix": databaseURL[:30] + "...",
		"url_suffix": "..." + databaseURL[len(databaseURL)-20:],
	})

	// 檢查其他可選環境變數
	port := os.Getenv("PORT")
	if port == "" {
		d.addResult("PORT", "WARN", "PORT 環境變數未設定，使用預設值 8080", "在 App Platform 中通常會自動設定", nil)
	} else {
		d.addResult("PORT", "PASS", fmt.Sprintf("PORT 設定為 %s", port), "", nil)
	}

	discordWebhook := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordWebhook == "" {
		d.addResult("DISCORD_WEBHOOK_URL", "WARN", "Discord Webhook 未設定", "如需通知功能，請設定此環境變數", nil)
	} else {
		d.addResult("DISCORD_WEBHOOK_URL", "PASS", "Discord Webhook 已設定", "", nil)
	}
}

// checkDatabaseURL 檢查資料庫 URL 詳細資訊
func (d *ConnectionDiagnostic) checkDatabaseURL() {
	fmt.Println("🔗 分析資料庫 URL...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		d.addResult("URL 解析", "FAIL", fmt.Sprintf("無法解析 DATABASE_URL: %v", err), "檢查 URL 格式是否正確", nil)
		return
	}

	d.addResult("URL 解析", "PASS", "DATABASE_URL 解析成功", "", map[string]interface{}{
		"scheme":   parsedURL.Scheme,
		"host":     parsedURL.Host,
		"path":     parsedURL.Path,
		"username": parsedURL.User.Username(),
	})

	// 檢查主機名是否為 Supabase
	if strings.Contains(parsedURL.Host, "supabase.co") {
		d.addResult("Supabase 主機", "PASS", "確認為 Supabase 主機", "", map[string]interface{}{
			"host": parsedURL.Host,
		})
	} else {
		d.addResult("Supabase 主機", "WARN", "非 Supabase 主機", "確認是否使用正確的 Supabase 連線字串", map[string]interface{}{
			"host": parsedURL.Host,
		})
	}

	// 檢查 SSL 參數
	queryParams := parsedURL.Query()
	sslMode := queryParams.Get("sslmode")
	if sslMode == "" {
		d.addResult("SSL 模式", "WARN", "未指定 SSL 模式", "建議添加 ?sslmode=require 到 URL 末尾", nil)
	} else {
		d.addResult("SSL 模式", "PASS", fmt.Sprintf("SSL 模式: %s", sslMode), "", map[string]interface{}{
			"sslmode": sslMode,
		})
	}
}

// testNetworkConnectivity 測試網路連線
func (d *ConnectionDiagnostic) testNetworkConnectivity() {
	fmt.Println("🌐 測試網路連線...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		return
	}

	// 測試 TCP 連線
	host := parsedURL.Host
	if !strings.Contains(host, ":") {
		host += ":5432" // 預設 PostgreSQL 埠
	}

	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		d.addResult("TCP 連線", "FAIL", fmt.Sprintf("無法連接到 %s: %v", host, err), "檢查網路連線和防火牆設定", map[string]interface{}{
			"host":  host,
			"error": err.Error(),
		})
		return
	}
	conn.Close()

	d.addResult("TCP 連線", "PASS", fmt.Sprintf("成功連接到 %s", host), "", map[string]interface{}{
		"host": host,
	})
}

// testSSLModes 測試不同 SSL 模式
func (d *ConnectionDiagnostic) testSSLModes() {
	fmt.Println("🔒 測試 SSL 連線模式...")

	baseURL := os.Getenv("DATABASE_URL")
	if baseURL == "" {
		return
	}

	// 移除現有的 SSL 參數
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return
	}

	query := parsedURL.Query()
	query.Del("sslmode")
	parsedURL.RawQuery = query.Encode()
	baseURLClean := parsedURL.String()

	// 測試不同 SSL 模式
	sslModes := []string{"require", "prefer", "disable"}

	for _, mode := range sslModes {
		testURL := baseURLClean
		if strings.Contains(testURL, "?") {
			testURL += "&sslmode=" + mode
		} else {
			testURL += "?sslmode=" + mode
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		conn, err := pgx.Connect(ctx, testURL)
		cancel()

		if err != nil {
			d.addResult(fmt.Sprintf("SSL 模式 %s", mode), "FAIL", fmt.Sprintf("連線失敗: %v", err), "", map[string]interface{}{
				"sslmode": mode,
				"error":   err.Error(),
			})
			continue
		}

		err = conn.Ping(ctx)
		conn.Close(ctx)

		if err != nil {
			d.addResult(fmt.Sprintf("SSL 模式 %s", mode), "FAIL", fmt.Sprintf("Ping 失敗: %v", err), "", map[string]interface{}{
				"sslmode": mode,
				"error":   err.Error(),
			})
		} else {
			d.addResult(fmt.Sprintf("SSL 模式 %s", mode), "PASS", "連線和 Ping 成功", "", map[string]interface{}{
				"sslmode": mode,
			})
		}
	}
}

// testConnectionPools 測試連線池設定
func (d *ConnectionDiagnostic) testConnectionPools() {
	fmt.Println("🏊 測試連線池設定...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return
	}

	// 測試不同連線池大小
	poolSizes := []int{1, 5, 10}

	for _, size := range poolSizes {
		config, err := pgxpool.ParseConfig(databaseURL)
		if err != nil {
			d.addResult(fmt.Sprintf("連線池 %d", size), "FAIL", fmt.Sprintf("配置解析失敗: %v", err), "", nil)
			continue
		}

		config.MaxConns = int32(size)
		config.MinConns = 1
		config.MaxConnLifetime = time.Hour
		config.MaxConnIdleTime = time.Minute * 30

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		pool, err := pgxpool.ConnectConfig(ctx, config)
		cancel()

		if err != nil {
			d.addResult(fmt.Sprintf("連線池 %d", size), "FAIL", fmt.Sprintf("連線池建立失敗: %v", err), "", map[string]interface{}{
				"pool_size": size,
				"error":     err.Error(),
			})
			continue
		}

		err = pool.Ping(ctx)
		pool.Close()

		if err != nil {
			d.addResult(fmt.Sprintf("連線池 %d", size), "FAIL", fmt.Sprintf("連線池 Ping 失敗: %v", err), "", map[string]interface{}{
				"pool_size": size,
				"error":     err.Error(),
			})
		} else {
			d.addResult(fmt.Sprintf("連線池 %d", size), "PASS", "連線池運作正常", "", map[string]interface{}{
				"pool_size": size,
			})
		}
	}
}

// testDatabaseOperations 測試資料庫操作
func (d *ConnectionDiagnostic) testDatabaseOperations() {
	fmt.Println("💾 測試資料庫操作...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		d.addResult("資料庫連線", "FAIL", fmt.Sprintf("無法建立連線: %v", err), "檢查 DATABASE_URL 和網路連線", nil)
		return
	}
	defer pool.Close()

	// 測試基本查詢
	var version string
	err = pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		d.addResult("基本查詢", "FAIL", fmt.Sprintf("版本查詢失敗: %v", err), "檢查資料庫權限", nil)
	} else {
		d.addResult("基本查詢", "PASS", "版本查詢成功", "", map[string]interface{}{
			"version": version[:50] + "...",
		})
	}

	// 測試資料表查詢
	rows, err := pool.Query(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' LIMIT 5")
	if err != nil {
		d.addResult("資料表查詢", "FAIL", fmt.Sprintf("資料表查詢失敗: %v", err), "檢查資料庫權限和結構", nil)
	} else {
		tableCount := 0
		for rows.Next() {
			tableCount++
		}
		rows.Close()
		d.addResult("資料表查詢", "PASS", fmt.Sprintf("找到 %d 個資料表", tableCount), "", map[string]interface{}{
			"table_count": tableCount,
		})
	}

	// 檢查必要資料表
	requiredTables := []string{"key_ranges", "found_wallets"}
	for _, table := range requiredTables {
		var exists bool
		err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)", table).Scan(&exists)
		if err != nil {
			d.addResult(fmt.Sprintf("資料表 %s", table), "FAIL", fmt.Sprintf("檢查失敗: %v", err), "", nil)
		} else if !exists {
			d.addResult(fmt.Sprintf("資料表 %s", table), "FAIL", "資料表不存在", "執行 create_tables.sql 建立資料表", nil)
		} else {
			d.addResult(fmt.Sprintf("資料表 %s", table), "PASS", "資料表存在", "", nil)
		}
	}
}

// checkSupabaseSpecific 檢查 Supabase 特定設定
func (d *ConnectionDiagnostic) checkSupabaseSpecific() {
	fmt.Println("🔧 檢查 Supabase 特定設定...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return
	}

	parsedURL, err := url.Parse(databaseURL)
	if err != nil {
		return
	}

	// 檢查是否為 Supabase
	if !strings.Contains(parsedURL.Host, "supabase.co") {
		d.addResult("Supabase 檢查", "WARN", "非 Supabase 主機，跳過 Supabase 特定檢查", "", nil)
		return
	}

	// 檢查專案 ID
	hostParts := strings.Split(parsedURL.Host, ".")
	if len(hostParts) >= 3 && strings.HasPrefix(hostParts[0], "db") {
		projectID := hostParts[1]
		d.addResult("Supabase 專案 ID", "PASS", fmt.Sprintf("專案 ID: %s", projectID), "", map[string]interface{}{
			"project_id": projectID,
		})
	} else {
		d.addResult("Supabase 專案 ID", "WARN", "無法解析專案 ID", "檢查 URL 格式", nil)
	}

	// 檢查連線埠
	port := parsedURL.Port()
	if port == "" || port == "5432" {
		d.addResult("Supabase 埠號", "PASS", "使用標準 PostgreSQL 埠 5432", "", nil)
	} else {
		d.addResult("Supabase 埠號", "WARN", fmt.Sprintf("非標準埠號: %s", port), "確認是否為正確的 Supabase 連線埠", nil)
	}

	// 檢查使用者名稱
	username := parsedURL.User.Username()
	if username == "postgres" {
		d.addResult("Supabase 使用者", "PASS", "使用標準 postgres 使用者", "", nil)
	} else {
		d.addResult("Supabase 使用者", "WARN", fmt.Sprintf("非標準使用者: %s", username), "Supabase 通常使用 postgres 作為使用者名稱", nil)
	}
}

// addResult 添加診斷結果
func (d *ConnectionDiagnostic) addResult(test, status, message, suggestion string, details map[string]interface{}) {
	d.Results = append(d.Results, DiagnosticResult{
		Test:       test,
		Status:     status,
		Message:    message,
		Suggestion: suggestion,
		Details:    details,
	})
}

// printReport 輸出診斷報告
func (d *ConnectionDiagnostic) printReport() {
	fmt.Println("\n📊 診斷報告")
	fmt.Println("============")

	passCount := 0
	failCount := 0
	warnCount := 0

	for _, result := range d.Results {
		var icon string
		switch result.Status {
		case "PASS":
			icon = "✅"
			passCount++
		case "FAIL":
			icon = "❌"
			failCount++
		case "WARN":
			icon = "⚠️"
			warnCount++
		}

		fmt.Printf("%s %s: %s\n", icon, result.Test, result.Message)
		if result.Suggestion != "" {
			fmt.Printf("   💡 建議: %s\n", result.Suggestion)
		}
		if result.Details != nil && len(result.Details) > 0 {
			fmt.Printf("   📋 詳細資訊: %+v\n", result.Details)
		}
		fmt.Println()
	}

	fmt.Printf("📈 總結: %d 通過, %d 失敗, %d 警告\n", passCount, failCount, warnCount)
}

// provideFixes 提供修復建議
func (d *ConnectionDiagnostic) provideFixes() {
	fmt.Println("\n🔧 修復建議")
	fmt.Println("============")

	hasFailures := false
	for _, result := range d.Results {
		if result.Status == "FAIL" {
			hasFailures = true
			break
		}
	}

	if !hasFailures {
		fmt.Println("🎉 所有關鍵測試都通過了！如果仍有連線問題，請檢查：")
		fmt.Println("1. Digital Ocean App Platform 的日誌")
		fmt.Println("2. Supabase 專案的網路設定")
		fmt.Println("3. 應用程式的錯誤處理邏輯")
		return
	}

	fmt.Println("🚨 發現問題，請按照以下步驟修復：")
	fmt.Println()

	// 環境變數問題
	for _, result := range d.Results {
		if result.Status == "FAIL" && strings.Contains(result.Test, "DATABASE_URL") {
			fmt.Println("1. 設定 DATABASE_URL 環境變數：")
			fmt.Println("   - 登入 Digital Ocean 控制台")
			fmt.Println("   - 進入 App Platform > 你的應用程式 > Settings > Environment Variables")
			fmt.Println("   - 添加 DATABASE_URL 變數，值為你的 Supabase 連線字串")
			fmt.Println("   - 格式: postgresql://postgres:password@db.project-id.supabase.co:5432/postgres")
			fmt.Println()
			break
		}
	}

	// 網路連線問題
	for _, result := range d.Results {
		if result.Status == "FAIL" && strings.Contains(result.Test, "TCP") {
			fmt.Println("2. 解決網路連線問題：")
			fmt.Println("   - 檢查 Supabase 專案是否暫停")
			fmt.Println("   - 確認 Supabase 專案的網路設定")
			fmt.Println("   - 檢查 Digital Ocean 的出站網路連線")
			fmt.Println()
			break
		}
	}

	// SSL 問題
	for _, result := range d.Results {
		if result.Status == "FAIL" && strings.Contains(result.Test, "SSL") {
			fmt.Println("3. 修復 SSL 連線問題：")
			fmt.Println("   - 在 DATABASE_URL 末尾添加 ?sslmode=require")
			fmt.Println("   - 如果仍有問題，嘗試 ?sslmode=prefer")
			fmt.Println("   - 確保 Digital Ocean 支援 SSL 連線")
			fmt.Println()
			break
		}
	}

	// 資料表問題
	for _, result := range d.Results {
		if result.Status == "FAIL" && strings.Contains(result.Test, "資料表") {
			fmt.Println("4. 建立必要的資料表：")
			fmt.Println("   - 執行 create_tables.sql 檔案")
			fmt.Println("   - 或使用 setup_database.go 工具")
			fmt.Println("   - 確保資料庫使用者有建立資料表的權限")
			fmt.Println()
			break
		}
	}

	fmt.Println("📞 如果問題持續存在，請檢查：")
	fmt.Println("   - Digital Ocean App Platform 日誌")
	fmt.Println("   - Supabase 專案儀表板")
	fmt.Println("   - 網路連線和防火牆設定")
}
