# 🚀 Supabase Setup Guide for Bitcoin Scanner

## 🎯 為什麼選擇 Supabase？

- ✅ **免費開始**: 每月 500MB 儲存和 2GB 傳輸
- ✅ **PostgreSQL**: 完全兼容的 PostgreSQL 資料庫
- ✅ **自動 SSL**: 內建安全連接
- ✅ **簡單設置**: 幾分鐘內完成配置
- ✅ **全球 CDN**: 快速訪問速度

## 📋 Step 1: 創建 Supabase 項目

### 1.1 註冊 Supabase
1. 訪問 [supabase.com](https://supabase.com)
2. 點擊 "Start your project"
3. 使用 GitHub 或 Email 註冊

### 1.2 創建新項目
1. 點擊 "New Project"
2. 選擇組織（或創建新的）
3. 填寫項目詳情：
   - **Name**: `bitcoin-scanner`
   - **Database Password**: 生成強密碼並**保存**
   - **Region**: 選擇 `West US (North California)` （最接近 blockchain.info API）
4. 點擊 "Create new project"

等待 2-3 分鐘完成初始化...

## 📋 Step 2: 獲取連接信息

項目創建完成後：

### 2.1 在 Supabase Dashboard
1. 進入項目 Dashboard
2. 點擊左側菜單 "Settings" → "Database"
3. 滾動到 "Connection parameters" 部分

### 2.2 重要信息記錄
```
Host: db.xxxxxxxxxxxxxx.supabase.co
Database name: postgres
Port: 5432
User: postgres
Password: [您設置的密碼]
```

### 2.3 連接字符串格式
```
postgresql://postgres:[PASSWORD]@db.[PROJECT-ID].supabase.co:5432/postgres
```

## 🔧 Step 3: 配置環境變數

### 3.1 在本地測試 (.env 文件)
創建或更新 `.env` 文件：

```bash
# Supabase PostgreSQL 連接
DATABASE_URL=postgresql://postgres:YOUR_PASSWORD@db.YOUR_PROJECT_ID.supabase.co:5432/postgres

# Discord Webhook (可選)
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_TOKEN

# 應用端口 (Digital Ocean 自動設置)
PORT=8080
```

**替換以下值**:
- `YOUR_PASSWORD`: 您的 Supabase 資料庫密碼
- `YOUR_PROJECT_ID`: 您的 Supabase 項目 ID

### 3.2 在 Digital Ocean App Platform
1. 登入 Digital Ocean Console
2. 進入您的應用
3. 點擊 "Settings" → "App-Level Environment Variables"
4. 添加環境變數：

```
KEY: DATABASE_URL
VALUE: postgresql://postgres:YOUR_PASSWORD@db.YOUR_PROJECT_ID.supabase.co:5432/postgres
ENCRYPT: ✅ (勾選加密)

KEY: DISCORD_WEBHOOK_URL  
VALUE: https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_TOKEN
ENCRYPT: ✅ (勾選加密)

KEY: PORT
VALUE: 8080
ENCRYPT: ❌ (不需要加密)
```

### 3.3 實際範例
假設您的項目 ID 是 `abcdefghijklmnop`，密碼是 `mysecretpassword123`：

```bash
DATABASE_URL=postgresql://postgres:mysecretpassword123@db.abcdefghijklmnop.supabase.co:5432/postgres
```

## 🗄️ Step 4: 創建資料庫表

### 4.1 使用 Supabase SQL Editor
1. 在 Supabase Dashboard 中，點擊左側 "SQL Editor"
2. 點擊 "New query"
3. 貼上以下 SQL 代碼：

```sql
-- 創建私鑰範圍表
CREATE TABLE key_ranges (
    id SERIAL PRIMARY KEY,
    start_key_hex TEXT NOT NULL,
    end_key_hex TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    worker_id TEXT,
    claimed_at TIMESTAMP,
    completed_at TIMESTAMP,
    UNIQUE(start_key_hex, end_key_hex)
);

-- 創建發現錢包表
CREATE TABLE found_wallets (
    id SERIAL PRIMARY KEY,
    private_key_wif TEXT UNIQUE NOT NULL,
    address TEXT NOT NULL,
    balance_satoshi BIGINT NOT NULL,
    worker_id TEXT,
    found_at TIMESTAMP DEFAULT NOW()
);

-- 創建性能索引
CREATE INDEX idx_key_ranges_status ON key_ranges(status);
CREATE INDEX idx_key_ranges_claimed_at ON key_ranges(claimed_at);
CREATE INDEX idx_found_wallets_address ON found_wallets(address);
CREATE INDEX idx_found_wallets_found_at ON found_wallets(found_at);

-- 插入測試資料來驗證連接
INSERT INTO key_ranges (start_key_hex, end_key_hex, status) 
VALUES ('1000', '2000', 'pending');

-- 驗證表創建成功
SELECT 'Tables created successfully!' as status;
```

4. 點擊 "Run" 執行 SQL

### 4.2 驗證表創建
在 SQL Editor 中執行：

```sql
-- 檢查所有表
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' 
ORDER BY table_name;

-- 檢查測試資料
SELECT * FROM key_ranges LIMIT 5;
```

## 🔄 Step 5: 填充工作範圍

### 5.1 本地填充（推薦）
```bash
# 設置環境變數（替換為您的實際值）
export DATABASE_URL="postgresql://postgres:YOUR_PASSWORD@db.YOUR_PROJECT_ID.supabase.co:5432/postgres"

# 測試連接
psql $DATABASE_URL -c "SELECT version();"

# 構建並運行填充工具
go build -tags populate -o populate_ranges .
./populate_ranges
```

### 5.2 直接在 Supabase 填充
也可以直接在 Supabase SQL Editor 中執行填充：

```sql
-- 簡化版本的範圍填充
DO $$
DECLARE
    i INTEGER;
    start_key BIGINT;
    end_key BIGINT;
    range_size INTEGER := 1000000; -- 1 million keys per range
BEGIN
    FOR i IN 1..100 LOOP
        start_key := (i - 1) * range_size + 268435456; -- Start from 16^7
        end_key := start_key + range_size - 1;
        
        INSERT INTO key_ranges (start_key_hex, end_key_hex, status)
        VALUES (to_hex(start_key), to_hex(end_key), 'pending')
        ON CONFLICT (start_key_hex, end_key_hex) DO NOTHING;
    END LOOP;
    
    RAISE NOTICE 'Created % work ranges', i;
END $$;
```

## ✅ Step 6: 測試部署

### 6.1 更新 Digital Ocean App
1. 確保環境變數已正確設置
2. 重新部署應用：
   - 在 Digital Ocean Console 中點擊 "Actions" → "Force Rebuild and Deploy"

### 6.2 驗證連接
部署完成後，檢查以下端點：

```bash
# 健康檢查
curl https://your-app-name.ondigitalocean.app/health

# 詳細狀態  
curl https://your-app-name.ondigitalocean.app/status

# Web 界面
# 在瀏覽器中打開: https://your-app-name.ondigitalocean.app/
```

預期看到：
```json
{
  "status": "healthy",
  "message": "All systems operational",
  "worker_id": "worker-xxx-123",
  "timestamp": "2025-08-18T06:30:00Z"
}
```

## 🔍 Step 7: 監控和管理

### 7.1 Supabase Dashboard 監控
1. **Database**: 查看連接數、查詢性能
2. **Logs**: 實時查看資料庫日誌
3. **API**: 查看 API 使用統計

### 7.2 查詢進度
在 Supabase SQL Editor 中：

```sql
-- 檢查工作進度
SELECT 
    status, 
    COUNT(*) as count,
    ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER(), 2) as percentage
FROM key_ranges 
GROUP BY status
ORDER BY status;

-- 檢查最新活動
SELECT 
    worker_id,
    COUNT(*) as ranges_processed,
    MAX(completed_at) as last_completed
FROM key_ranges 
WHERE status = 'completed'
GROUP BY worker_id
ORDER BY ranges_processed DESC;

-- 檢查發現的錢包（希望有結果！）
SELECT 
    address,
    balance_satoshi,
    balance_satoshi / 100000000.0 as balance_btc,
    found_at
FROM found_wallets
ORDER BY found_at DESC;
```

## 🚨 故障排除

### 常見錯誤及解決方案

**1. 連接被拒絕**
```bash
# 檢查 DATABASE_URL 格式
echo $DATABASE_URL
# 應該是: postgresql://postgres:password@db.xxx.supabase.co:5432/postgres
```

**2. 密碼認證失敗**
- 確認密碼正確（不包含特殊字符問題）
- 在 Supabase Dashboard 重置密碼

**3. SSL 錯誤**
- Supabase 自動使用 SSL，無需額外配置
- 如果有問題，可以添加 `?sslmode=require`

**4. 權限錯誤**
- Supabase 的 `postgres` 用戶默認有所有權限
- 如果有問題，檢查表是否在正確的 schema 中

## 💰 Supabase 定價

### 免費層 (Free Tier)
- ✅ 500MB 資料庫儲存
- ✅ 2GB 傳輸量/月
- ✅ 無限 API 請求
- ✅ 最多 2 個項目

### Pro 計劃 ($25/月)
- ✅ 8GB 資料庫儲存
- ✅ 250GB 傳輸量/月  
- ✅ 無限項目
- ✅ 每日自動備份

對於此項目，**免費層通常就足夠了**！

## 🎉 完成！

現在您的比特幣掃描器應該能夠：

1. ✅ 成功連接到 Supabase PostgreSQL
2. ✅ 通過 Digital Ocean 健康檢查
3. ✅ 開始掃描比特幣私鑰
4. ✅ 在發現有餘額時發送 Discord 通知
5. ✅ 提供 Web 界面監控狀態

🚀 **開始掃描，尋找比特幣寶藏吧！**

---

## 快速參考

### 環境變數模板
```bash
DATABASE_URL=postgresql://postgres:YOUR_PASSWORD@db.YOUR_PROJECT_ID.supabase.co:5432/postgres
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_TOKEN
PORT=8080
```

### 重要鏈接
- [Supabase Dashboard](https://app.supabase.com/)
- [Digital Ocean Apps](https://cloud.digitalocean.com/apps)
- [項目狀態](https://your-app-name.ondigitalocean.app/)