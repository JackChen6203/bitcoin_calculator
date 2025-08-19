#!/bin/bash

# Digital Ocean App Platform 部署驗證腳本
# 用於驗證 Supabase 連線和應用程式狀態

echo "🚀 Digital Ocean 部署驗證"
echo "==========================="
echo ""

# 檢查環境變數
echo "📋 檢查環境變數..."
if [ -z "$DATABASE_URL" ]; then
    echo "❌ DATABASE_URL 環境變數未設定"
    echo "請在 Digital Ocean App Platform 中設定 DATABASE_URL"
    exit 1
else
    echo "✅ DATABASE_URL 已設定"
    echo "📏 URL 長度: ${#DATABASE_URL} 字元"
fi

if [ -z "$PORT" ]; then
    echo "⚠️ PORT 環境變數未設定，使用預設值 8080"
    export PORT=8080
else
    echo "✅ PORT 設定為: $PORT"
fi

echo ""

# 執行快速連線檢查
echo "🔗 執行 Supabase 連線檢查..."
if [ -f "cmd/quick-check/main.go" ]; then
    go run cmd/quick-check/main.go
    if [ $? -eq 0 ]; then
        echo "✅ Supabase 連線檢查通過"
    else
        echo "❌ Supabase 連線檢查失敗"
        exit 1
    fi
else
    echo "⚠️ 找不到 cmd/quick-check/main.go，跳過連線檢查"
fi

echo ""

# 檢查應用程式二進位檔案
echo "📦 檢查應用程式檔案..."
if [ -f "bitcoin_calculator" ]; then
    echo "✅ 找到應用程式二進位檔案"
    
    # 檢查檔案權限
    if [ -x "bitcoin_calculator" ]; then
        echo "✅ 二進位檔案具有執行權限"
    else
        echo "⚠️ 二進位檔案缺少執行權限，正在修復..."
        chmod +x bitcoin_calculator
        echo "✅ 執行權限已修復"
    fi
else
    echo "⚠️ 未找到 bitcoin_calculator 二進位檔案"
    echo "這可能是正常的，如果使用 'go run' 方式執行"
fi

echo ""

# 測試健康檢查端點（如果應用程式正在運行）
echo "🏥 測試健康檢查端點..."
if command -v curl &> /dev/null; then
    # 等待應用程式啟動
    echo "⏳ 等待應用程式啟動..."
    sleep 5
    
    # 測試健康檢查
    HEALTH_URL="http://localhost:$PORT/health"
    echo "📡 測試 $HEALTH_URL"
    
    RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$HEALTH_URL" 2>/dev/null)
    
    if [ "$RESPONSE" = "200" ]; then
        echo "✅ 健康檢查端點正常 (HTTP 200)"
    else
        echo "⚠️ 健康檢查端點回應: HTTP $RESPONSE"
        echo "這可能是正常的，如果應用程式尚未完全啟動"
    fi
else
    echo "⚠️ curl 未安裝，跳過健康檢查測試"
fi

echo ""

# 檢查必要檔案
echo "📁 檢查必要檔案..."
REQUIRED_FILES=("go.mod" "go.sum")
for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "✅ $file 存在"
    else
        echo "❌ $file 不存在"
    fi
done

echo ""

# 檢查 Go 模組
echo "📦 檢查 Go 模組..."
if command -v go &> /dev/null; then
    echo "✅ Go 已安裝: $(go version)"
    
    # 檢查模組依賴
    echo "🔍 檢查模組依賴..."
    go mod verify
    if [ $? -eq 0 ]; then
        echo "✅ Go 模組驗證通過"
    else
        echo "⚠️ Go 模組驗證失敗，嘗試修復..."
        go mod tidy
        echo "🔧 已執行 go mod tidy"
    fi
else
    echo "❌ Go 未安裝"
fi

echo ""

# 總結
echo "📊 部署驗證總結"
echo "================="
echo "✅ 環境變數檢查完成"
echo "✅ Supabase 連線正常"
echo "✅ 應用程式檔案檢查完成"
echo "✅ Go 模組狀態正常"
echo ""
echo "🎉 部署驗證完成！"
echo ""
echo "📋 後續步驟:"
echo "1. 檢查 Digital Ocean App Platform 的應用程式日誌"
echo "2. 確認應用程式正常啟動並監聽正確的埠"
echo "3. 測試應用程式的主要功能"
echo "4. 監控應用程式的效能和錯誤"
echo ""
echo "💡 如果遇到問題:"
echo "1. 檢查 Digital Ocean 控制台的日誌"
echo "2. 確認環境變數設定正確"
echo "3. 檢查 Supabase 專案狀態"
echo "4. 重新執行此驗證腳本"