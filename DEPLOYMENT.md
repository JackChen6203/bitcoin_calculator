# 🚀 Digital Ocean 部署指南

## 📋 預備要求

1. Digital Ocean 帳戶
2. PostgreSQL 資料庫（可使用 Digital Ocean Managed Database）
3. Discord Webhook URL（可選）

## 🛠️ 部署步驟

### 方法 1：使用 Digital Ocean App Platform（推薦）

#### 1. 準備環境變數

在 Digital Ocean App Platform 中設置以下環境變數：

```bash
# 必需
DATABASE_URL=postgres://username:password@host:port/database?sslmode=require

# 可選（Discord 通知）
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_TOKEN

# 系統變數（自動設置）
PORT=8080
```

#### 2. 創建 Digital Ocean App

**使用 App Spec（推薦）**:
```bash
# 使用項目中的 .do/app.yaml 文件
doctl apps create --spec .do/app.yaml
```

**或通過 Web 界面**:
1. 登入 Digital Ocean Console
2. 進入 App Platform
3. 創建新應用
4. 連接 GitHub 倉庫: `JackChen6203/bitcoin_calculator`
5. 選擇分支: `master`
6. 設置環境變數
7. 部署

#### 3. 設置資料庫

**使用 Digital Ocean Managed Database**:
```bash
# 創建 PostgreSQL 資料庫
doctl databases create bitcoin-db --engine pg --region sgp1 --size db-s-1vcpu-1gb
```

**手動設置資料庫結構**:
```sql
-- 連接到資料庫後執行
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

CREATE TABLE found_wallets (
    id SERIAL PRIMARY KEY,
    private_key_wif TEXT UNIQUE NOT NULL,
    address TEXT NOT NULL,
    balance_satoshi BIGINT NOT NULL,
    worker_id TEXT,
    found_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_key_ranges_status ON key_ranges(status);
CREATE INDEX idx_found_wallets_address ON found_wallets(address);
```

#### 4. 初始化工作範圍

部署後，需要填充私鑰範圍：

```bash
# 本地運行 populate 工具
export DATABASE_URL="your_database_url"
go build -tags populate -o populate_ranges .
./populate_ranges
```

### 方法 2：使用 Docker Droplet

#### 1. 創建 Droplet
```bash
# 推薦規格：2GB RAM, 1 vCPU, 舊金山區域
doctl compute droplet create bitcoin-scanner \
  --region sfo3 \
  --image docker-20-04 \
  --size s-2vcpu-2gb \
  --ssh-keys YOUR_SSH_KEY_ID
```

#### 2. 部署容器
```bash
# SSH 到 Droplet
ssh root@your_droplet_ip

# 克隆代碼
git clone https://github.com/JackChen6203/bitcoin_calculator.git
cd bitcoin_calculator

# 構建 Docker 映像
docker build -t bitcoin-scanner .

# 運行容器
docker run -d \
  --name bitcoin-scanner \
  --restart unless-stopped \
  -p 8080:8080 \
  -e DATABASE_URL="your_database_url" \
  -e DISCORD_WEBHOOK_URL="your_webhook_url" \
  bitcoin-scanner
```

## 📊 監控和管理

### 健康檢查端點

應用程序提供以下 HTTP 端點：

- **`/health`** - 健康檢查（用於 Digital Ocean 監控）
- **`/status`** - 詳細狀態信息
- **`/`** - 基本信息頁面

### 檢查應用狀態

```bash
# 檢查健康狀態
curl https://your-app-url/health

# 檢查詳細狀態
curl https://your-app-url/status
```

### 查看日誌

**Digital Ocean App Platform**:
```bash
# 使用 doctl CLI
doctl apps logs YOUR_APP_ID

# 或通過 Web 控制台查看
```

**Docker Droplet**:
```bash
# 查看容器日誌
docker logs -f bitcoin-scanner
```

### 資料庫監控

```sql
-- 檢查工作進度
SELECT 
    status, 
    COUNT(*) as count,
    ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER(), 2) as percentage
FROM key_ranges 
GROUP BY status;

-- 檢查發現的錢包
SELECT 
    COUNT(*) as total_wallets,
    SUM(balance_satoshi) as total_balance_satoshi
FROM found_wallets;

-- 檢查 Worker 性能
SELECT 
    worker_id,
    COUNT(*) as processed_ranges,
    MAX(completed_at) as last_activity
FROM key_ranges 
WHERE status = 'completed' 
GROUP BY worker_id 
ORDER BY processed_ranges DESC;
```

## 🔧 故障排除

### 常見問題

**1. 應用啟動失敗**
```bash
# 檢查環境變數
echo $DATABASE_URL
echo $DISCORD_WEBHOOK_URL

# 檢查資料庫連接
psql $DATABASE_URL -c "SELECT 1"
```

**2. 健康檢查失敗**
```bash
# 檢查端口是否正確監聽
curl localhost:8080/health

# 檢查防火牆設置
ufw status
```

**3. Discord 通知不工作**
```bash
# 測試 Discord Webhook
curl -X POST $DISCORD_WEBHOOK_URL \
  -H "Content-Type: application/json" \
  -d '{"content": "Test message"}'
```

### 性能優化

**擴展 Workers**:
```bash
# 在多個 Droplet 上運行
# 每個 Worker 會自動從資料庫獲取不同的工作範圍
```

**調整並發數**:
```go
// 在 main.go 中修改
const concurrency = 500  // 增加並發數（注意 API 限制）
```

**使用更快的 API**:
```go
// 考慮替換 blockchain.info 為其他 API
// 如 BlockCypher, Blockstream 等
```

## 🔒 安全考量

1. **環境變數**: 使用 Digital Ocean Secrets 管理敏感信息
2. **資料庫**: 啟用 SSL 連接和防火牆
3. **Discord**: 保護 Webhook URL 不被公開
4. **網路**: 限制入站連接只允許必要端口

## 💰 成本估算

### Digital Ocean App Platform
- **基本計劃**: $5-12/月
- **專業計劃**: $25-50/月（高性能）

### Managed Database
- **開發版**: $15/月（1GB RAM）
- **基礎版**: $50/月（2GB RAM）

### 總計
- **最小配置**: ~$20/月
- **推薦配置**: ~$60/月

## 🎯 推薦部署配置

### 生產環境
- **區域**: San Francisco 3 (sfo3) - 最接近 blockchain.info API
- **應用**: Professional plan, 2 containers
- **資料庫**: Basic plan, 2GB RAM
- **監控**: 啟用所有警報和通知

### 開發/測試環境
- **區域**: Singapore 1 (sgp1) - 亞洲區域
- **應用**: Basic plan, 1 container
- **資料庫**: Development plan, 1GB RAM

---

部署完成後，您的比特幣掃描器將自動開始工作，當發現有餘額的錢包時會發送 Discord 通知！