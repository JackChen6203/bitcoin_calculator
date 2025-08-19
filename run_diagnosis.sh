#!/bin/bash

# Supabase 連線診斷執行腳本
# 適用於 Digital Ocean App Platform

echo "🔍 開始 Supabase 連線診斷..."
echo "================================"

# 檢查 Go 是否安裝
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安裝，請先安裝 Go"
    exit 1
fi

# 檢查診斷工具是否存在
if [ ! -f "diagnose_connection.go" ]; then
    echo "❌ 找不到 diagnose_connection.go 檔案"
    exit 1
fi

# 執行診斷
echo "🚀 執行診斷工具..."
echo ""
go run diagnose_connection.go

# 檢查執行結果
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 診斷完成"
else
    echo ""
    echo "❌ 診斷執行失敗"
    exit 1
fi

# 提供後續建議
echo ""
echo "📋 後續步驟："
echo "1. 檢查上方的診斷結果"
echo "2. 根據修復建議進行調整"
echo "3. 在 Digital Ocean App Platform 中更新環境變數"
echo "4. 重新部署應用程式"
echo "5. 再次執行此診斷工具驗證修復結果"
echo ""
echo "💡 提示：如果在 Digital Ocean 上執行，確保已設定正確的 DATABASE_URL 環境變數"