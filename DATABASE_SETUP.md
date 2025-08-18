# 🗄️ Digital Ocean Database Setup Guide

## 🎯 Quick Setup for Bitcoin Scanner

### 方法 1：使用 Digital Ocean Managed Database（推薦）

#### 1. 創建 PostgreSQL 資料庫

**使用 Web 控制台**:
1. 登入 [Digital Ocean Console](https://cloud.digitalocean.com/)
2. 點擊 "Databases" → "Create Database Cluster"
3. 選擇設置：
   - **Engine**: PostgreSQL 14
   - **Plan**: Basic ($15/月) 或 Development ($4/月)
   - **Region**: 選擇與應用相同區域（建議 San Francisco）
   - **Database name**: `bitcoin_scanner`
   - **User**: `bitcoin_user`

**或使用 CLI**:
```bash
# 安裝 doctl
snap install doctl
# 或 brew install doctl

# 認證
doctl auth init

# 創建資料庫
doctl databases create bitcoin-scanner-db \
  --engine pg \
  --num-nodes 1 \
  --region sfo3 \
  --size db-s-1vcpu-1gb \
  --version 14
```

#### 2. 配置資料庫

等待資料庫創建完成（約 5-10 分鐘），然後：

```bash
# 獲取連接信息
doctl databases connection bitcoin-scanner-db

# 或在 Web 控制台中復制連接 URL
```

#### 3. 創建表結構

使用任何 PostgreSQL 客戶端連接到資料庫：

```bash
# 使用 psql 連接
psql "postgresql://username:password@host:25060/database?sslmode=require"
```

執行以下 SQL 命令：

```sql
-- 創建工作範圍表
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

-- 創建索引以提高查詢性能
CREATE INDEX idx_key_ranges_status ON key_ranges(status);
CREATE INDEX idx_key_ranges_claimed_at ON key_ranges(claimed_at);
CREATE INDEX idx_found_wallets_address ON found_wallets(address);
CREATE INDEX idx_found_wallets_found_at ON found_wallets(found_at);

-- 驗證表創建
\dt
```

#### 4. 配置 Digital Ocean App Platform

在 Digital Ocean App Platform 中設置環境變數：

1. 進入應用設置 → Environment Variables
2. 添加變數：

```
DATABASE_URL=postgresql://username:password@host:25060/database?sslmode=require
```

**注意**: 將 `username`, `password`, `host`, `database` 替換為實際值

### 方法 2：使用外部 PostgreSQL

如果您已有 PostgreSQL 伺服器：

```sql
-- 連接到您的 PostgreSQL
CREATE DATABASE bitcoin_scanner;

-- 創建用戶
CREATE USER bitcoin_user WITH PASSWORD 'your_secure_password';

-- 授予權限
GRANT ALL PRIVILEGES ON DATABASE bitcoin_scanner TO bitcoin_user;

-- 切換到新資料庫
\c bitcoin_scanner;

-- 授予 schema 權限
GRANT ALL ON SCHEMA public TO bitcoin_user;

-- 創建表（使用上面的 SQL）
```

## 🚀 填充工作範圍

資料庫設置完成後，需要填充私鑰範圍：

### 本地填充（推薦）

```bash
# 設置環境變數
export DATABASE_URL="postgresql://username:password@host:25060/database?sslmode=require"

# 構建並運行填充工具
go build -tags populate -o populate_ranges .
./populate_ranges
```

### 使用 Docker 填充

```bash
# 創建一次性容器來填充資料庫
docker run --rm \
  -e DATABASE_URL="your_database_url" \
  -v "$(pwd):/app" \
  -w /app \
  golang:1.22 \
  bash -c "go build -tags populate -o populate_ranges . && ./populate_ranges"
```

## 📊 驗證設置

### 檢查表創建

```sql
-- 檢查表是否存在
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public';

-- 檢查工作範圍數量
SELECT COUNT(*) as total_ranges FROM key_ranges;

-- 檢查範圍狀態分佈
SELECT status, COUNT(*) as count 
FROM key_ranges 
GROUP BY status;
```

### 檢查應用連接

部署後，訪問應用的健康檢查端點：

```bash
# 檢查健康狀態
curl https://your-app-name.ondigitalocean.app/health

# 檢查詳細狀態
curl https://your-app-name.ondigitalocean.app/status

# 或在瀏覽器中打開
# https://your-app-name.ondigitalocean.app/
```

預期響應：
```json
{
  "status": "healthy",
  "message": "All systems operational",
  "worker_id": "worker-hostname-123",
  "timestamp": "2025-08-18T06:30:00Z"
}
```

## 🔧 故障排除

### 常見問題

**1. 連接被拒絕**
```bash
# 檢查防火牆設置
# 確保 Digital Ocean 防火牆允許應用訪問資料庫
```

**2. SSL 證書錯誤**
```bash
# 確保連接字符串包含 sslmode=require
DATABASE_URL="postgresql://user:pass@host:25060/db?sslmode=require"
```

**3. 權限錯誤**
```sql
-- 授予更多權限
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO bitcoin_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO bitcoin_user;
```

### 性能調優

**連接池配置**:
```bash
# 在 DATABASE_URL 中添加連接池參數
DATABASE_URL="postgresql://user:pass@host:25060/db?sslmode=require&pool_max_conns=20"
```

**資料庫調優**:
```sql
-- 調整工作記憶體（需要超級用戶權限）
-- SET shared_buffers = '256MB';
-- SET work_mem = '64MB';

-- 分析表以優化查詢計劃
ANALYZE key_ranges;
ANALYZE found_wallets;
```

## 💰 成本估算

### Digital Ocean Managed Database

| 計劃 | vCPU | RAM | 儲存 | 價格/月 |
|------|------|-----|------|---------|
| Development | 1 | 1GB | 10GB | $15 |
| Basic | 1 | 1GB | 25GB | $25 |
| Basic | 1 | 2GB | 50GB | $50 |

### 推薦配置

**開發/測試**: Development 計劃 ($15/月)
- 適合測試和小規模掃描
- 1GB RAM，10GB 儲存

**生產**: Basic 2GB 計劃 ($50/月)  
- 支持多個 worker
- 更好的性能和儲存空間

## 🔒 安全最佳實踐

1. **使用強密碼**: 至少 16 字符，包含特殊字符
2. **啟用 SSL**: 始終使用 `sslmode=require`
3. **限制訪問**: 配置防火牆規則
4. **定期備份**: 啟用自動備份功能
5. **監控**: 設置資料庫性能監控

## 📈 擴展計劃

當需要更高性能時：

1. **垂直擴展**: 升級到更大的資料庫計劃
2. **讀取副本**: 添加只讀副本以分散查詢負載
3. **分區**: 按時間或範圍對大表進行分區
4. **索引優化**: 根據查詢模式添加更多索引

---

完成資料庫設置後，您的比特幣掃描器就可以開始工作了！🚀