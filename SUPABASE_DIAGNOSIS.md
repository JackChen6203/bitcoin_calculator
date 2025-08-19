# Supabase 連線診斷工具

## 🎯 目的
解決 Digital Ocean App Platform 上的 Supabase 連線問題。

## 🛠️ 可用工具

### 1. 快速檢查 (`quick_check.go`)
**用途**: 本地和部署後的快速連線驗證
```bash
go run quick_check.go
```

### 2. 詳細診斷 (`diagnose_connection.go`)
**用途**: 深度連線問題分析
```bash
go run diagnose_connection.go
```

### 3. 連線修復 (`fix_connection.go`)
**用途**: 測試不同連線配置，