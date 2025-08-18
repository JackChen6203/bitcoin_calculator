# Bitcoin Private Key Scanner

分散式比特幣私鑰掃描系統，用於尋找有餘額的比特幣錢包。當發現有餘額的錢包時，會自動推送通知到 Discord。

## 🚀 功能特色

- ✅ **分散式掃描**: 多個 worker 可同時運行
- ✅ **PostgreSQL 工作分配**: 原子性工作單元認領
- ✅ **並發處理**: 單個 worker 內部並發檢查私鑰
- ✅ **Discord 通知**: 發現有餘額錢包時自動通知
- ✅ **優雅關閉**: 支援信號處理，安全終止
- ✅ **持久化存儲**: 發現的錢包永久保存

## 📋 系統要求

- Go 1.22+
- PostgreSQL 12+
- Discord Webhook URL（可選）

## 🛠️ 安裝與配置

### 1. 克隆項目
```bash
git clone https://github.com/JackChen6203/bitcoin_calculator.git
cd bitcoin_calculator
```

### 2. 安裝依賴
```bash
go mod tidy
```

### 3. 設置環境變數
```bash
cp .env.example .env
# 編輯 .env 文件，填入您的配置
```

### 4. 配置資料庫
創建 PostgreSQL 資料庫和表：

```sql
-- 創建資料庫
CREATE DATABASE bitcoin_scanner;

-- 創建用戶
CREATE USER bitcoin_user WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE bitcoin_scanner TO bitcoin_user;

-- 連接到資料庫並創建表
\c bitcoin_scanner;

-- 工作範圍表
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

-- 發現的錢包表
CREATE TABLE found_wallets (
    id SERIAL PRIMARY KEY,
    private_key_wif TEXT UNIQUE NOT NULL,
    address TEXT NOT NULL,
    balance_satoshi BIGINT NOT NULL,
    worker_id TEXT,
    found_at TIMESTAMP DEFAULT NOW()
);

-- 創建索引
CREATE INDEX idx_key_ranges_status ON key_ranges(status);
CREATE INDEX idx_found_wallets_address ON found_wallets(address);
```

### 5. 設置 Discord Webhook（可選）

1. 進入您的 Discord 伺服器
2. 右鍵點擊頻道 → 編輯頻道 → 整合 → Webhook → 新增 Webhook
3. 複製 Webhook URL
4. 將 URL 添加到 `.env` 文件中的 `DISCORD_WEBHOOK_URL`

## 🚀 使用方法

### 1. 初始化工作範圍
```bash
# 構建 populate 工具
go build -tags populate -o populate_ranges .
./populate_ranges
```

### 2. 啟動掃描器
```bash
# 構建掃描器（默認）
go build -o bitcoin_scanner .
./bitcoin_scanner
```

### 3. 測試 Discord 通知
```bash
go run discord_test.go
```

## 📱 Discord 通知格式

當發現有餘額的錢包時，系統會發送以下格式的通知：

```
🚨 **發現有餘額的錢包！** 🚨
地址: 1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa
餘額: 50.00000000 BTC
私鑰: ||隱藏的私鑰||
```

通知包含：
- 🎯 **地址**: 比特幣錢包地址
- 💎 **餘額**: BTC 和 satoshi 格式
- 🔑 **私鑰**: WIF 格式（使用 Discord 隱藏標籤）
- ⏰ **發現時間**: 自動時間戳

## 🏗️ 系統架構

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Worker 1      │    │   PostgreSQL     │    │   Discord       │
│   (scanner)     │◄──►│   Database       │    │   Webhook       │
└─────────────────┘    │                  │    └─────────────────┘
                       │  ┌─────────────┐ │            ▲
┌─────────────────┐    │  │key_ranges   │ │            │
│   Worker 2      │◄──►│  │found_wallets│ │            │
│   (scanner)     │    │  └─────────────┘ │    ┌──────────────┐
└─────────────────┘    └──────────────────┘    │  Notification │
                                               │  on Balance   │
┌─────────────────┐    ┌──────────────────┐    │  Discovery    │
│   Worker N      │    │  blockchain.info │    └──────────────┘
│   (scanner)     │◄──►│   API Service    │
└─────────────────┘    └──────────────────┘
```

## ⚙️ 配置選項

### 環境變數

| 變數名 | 描述 | 必需 | 預設值 |
|--------|------|------|--------|
| `DATABASE_URL` | PostgreSQL 連接 URL | ✅ | - |
| `DISCORD_WEBHOOK_URL` | Discord Webhook URL | ❌ | - |

### 程式常數

```go
const (
    concurrency = 200  // 並發 worker 數量
    workerID = "worker-"  // Worker ID 前綴
)
```

## 🔧 部署建議

### Digital Ocean 部署
推薦使用美國西岸（舊金山）區域以最小化到 blockchain.info API 的延遲：

```bash
# 推薦配置
- 區域: San Francisco 3 (sfo3)
- 規格: 2GB RAM, 1 vCPU, 50GB SSD
- 成本: ~$12/月
```

### 系統服務配置
```bash
# 創建 systemd 服務
sudo tee /etc/systemd/system/bitcoin-scanner.service << EOF
[Unit]
Description=Bitcoin Private Key Scanner
After=network.target postgresql.service

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/bitcoin_calculator
ExecStart=/home/ubuntu/bitcoin_calculator/bitcoin_scanner
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable bitcoin-scanner
sudo systemctl start bitcoin-scanner
```

## 📊 監控和日誌

### 檢查運行狀態
```bash
# 服務狀態
sudo systemctl status bitcoin-scanner

# 實時日誌
sudo journalctl -u bitcoin-scanner -f

# 資料庫狀態
psql -U bitcoin_user -d bitcoin_scanner -c "
SELECT 
    status, 
    COUNT(*) as count,
    ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER(), 2) as percentage
FROM key_ranges 
GROUP BY status;
"
```

### 性能指標
```sql
-- 發現的錢包統計
SELECT 
    COUNT(*) as total_wallets,
    SUM(balance_satoshi) as total_balance_satoshi,
    AVG(balance_satoshi) as avg_balance_satoshi,
    MAX(balance_satoshi) as max_balance_satoshi
FROM found_wallets;

-- Worker 性能
SELECT 
    worker_id,
    COUNT(*) as processed_ranges,
    MIN(completed_at) as first_completion,
    MAX(completed_at) as last_completion
FROM key_ranges 
WHERE status = 'completed' 
GROUP BY worker_id 
ORDER BY processed_ranges DESC;
```

## ⚠️ 安全注意事項

1. **私鑰保護**: 發現的私鑰存儲在資料庫中，確保資料庫安全
2. **Discord 安全**: Webhook URL 包含敏感令牌，不要公開分享
3. **網路安全**: 在生產環境中使用 SSL/TLS 連接
4. **API 限制**: 尊重 blockchain.info API 的使用限制

## 🤝 貢獻

歡迎提交 Issue 和 Pull Request！

## 📄 授權

此項目僅供教育和研究目的使用。

---

**⚠️ 免責聲明**: 此工具僅用於教育目的。實際上找到有餘額的隨機私鑰的機率極其微小（約 2^256 分之一）。請負責任地使用此工具。