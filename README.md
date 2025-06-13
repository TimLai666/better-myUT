# 更好的校務系統 (Better myUT)

這是一個針對臺北市立大學校務系統的響應式設計優化代理伺服器。

## 功能特色

- **響應式設計**：自動為校務系統添加響應式 CSS，讓網站在手機和平板上也能正常瀏覽
- **會話維持**：保持與原始校務系統的登入狀態
- **表格優化**：針對校務系統的表格進行手機版優化
- **UI 美化**：改善按鈕、表單等元素的視覺效果

## 安裝與設定

### 1. 克隆專案
```bash
git clone https://github.com/TimLai666/better-myUT
cd better-myUT
```

### 2. 安裝依賴
```bash
go mod tidy
```

### 3. 設定環境變數
複製 `env.example` 為 `.env` 並填入您的資訊：
```bash
cp env.example .env
```

編輯 `.env` 文件：
```env
# 伺服器設定
# 內部監聽的埠號（容器內或本機）
PORT=8080

# 上游校務系統網址（通常保持預設即可）
TARGET_HOST=https://my.utaipei.edu.tw

# 對外公開的代理網址（*非常重要*）
# 用來在 HTML 中把所有重定向、超連結改寫成部署後的網域，
# 例如您透過 Nginx 反向代理到 https://your-domain.com 時：
PROXY_HOST=https://your-domain.com
```

## 使用方法

### 啟動伺服器
```bash
go run main.go
```

### 本地訪問
在瀏覽器中開啟：
```
http://localhost:8080/utaipei/index_sky.html
```

---

## Docker 快速部署

```bash
docker build -t better-myut .

# 例如要對外提供 https://example.com/ 服務，可用 80:8080 並設定 PROXY_HOST
docker run -d --name myut -p 80:8080 \
  -e PROXY_HOST=https://example.com \
  -e TARGET_HOST=https://my.utaipei.edu.tw \
  better-myut
```

若您已經有前端 Nginx / Traefik 等反向代理，可僅暴露內部埠，並在代理層設定對應路徑。

## 技術細節

### 架構說明
- **代理伺服器**：使用 Go 標準庫實現 HTTP 代理
- **會話管理**：使用 `cookiejar` 維持 cookie 會話
- **HTML 處理**：動態注入響應式 CSS 和優化代碼
- **環境變數**：使用 `godotenv` 管理配置

### 響應式設計特色
- 表格在手機版會轉換為卡片式佈局
- 自動調整字體大小和間距
- 優化表單元素的觸控體驗
- 改善導航菜單的手機顯示

## 故障排除

### 常見問題

**Q: 無法連接到校務系統**
A: 檢查網路連接和 `TARGET_HOST` 設定是否正確

**Q: 頁面顯示不正常**
A: 校務系統可能更新了結構，需要調整 CSS 或 HTML 處理邏輯

### 日誌查看
程式會輸出詳細的日誌訊息，包括：
- 伺服器啟動資訊
- 登入狀態
- 請求處理狀況

## 貢獻指南

歡迎提交 Issue 和 Pull Request 來改善這個專案！

## 授權條款

請參閱 LICENSE 文件。