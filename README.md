# 更好的校務系統（better-myUT）

> 讓臺北市立大學校務資訊系統在行動裝置上也能有更佳體驗！
>
> 本專案以 **Go 1.24** 與 **Gin Web Framework** 所撰寫的 **反向代理伺服器**，在不中斷既有功能的前提下，動態注入響應式 CSS/JS，並修正頁面快取、重定向與 Frameset 等問題。

---

## 功能總覽

| 分類 | 功能描述 |
| --- | --- |
| 響應式介面 | • 自動注入 `injected.css`，解除右鍵禁用並優化側邊選單（`#m_tree`）及功能按鈕外觀。<br/>• 自動為 `<td>` 加上 `data-label`，對應欄位名稱以便 CSS 於窄螢幕用 `::before` 顯示。 |
| 代理強化 | • 智慧重寫 `Location` / 內嵌 URL 以回到代理本身。<br/>• 內建 CookieJar 維持與上游（my.utaipei.edu.tw）的登入狀態。 |
| 快取控制 | • 自行覆寫 `Cache-Control` / `Pragma` / `Expires` 標頭與對應 HTML `<meta>`，確保前端永遠取得最新內容。 |
| 部署便利 | • 單一可執行檔（Windows/macOS/Linux）或透過 Docker image 快速啟動。 |

---

## 快速開始

### 1. 編譯

```bash
go build -o better-myUT main.go
```

### 2. 建立 `.env`

專案中提供 `env.example`，請將其改名為 `.env` 並填入相關資訊：

```dotenv
# 伺服器監聽埠（預設 8080）
PORT=8080

# 上游校務系統網址，理論上保持預設即可
TARGET_HOST=https://my.utaipei.edu.tw

# 部署後對外的完整網址（⚠️ 極度重要）
# 代理會利用此值把所有絕對網址/重定向改寫回自身
PROXY_HOST=https://your.domain.com
```

### 3. 執行

```bash
./better-myUT   # 或 go run main.go
```

瀏覽器進入 `http://localhost:8080/utaipei/index_sky.html`，即可看到行動版優化後的校務系統。

---

## Docker 部署

```bash
docker build -t better-myut .

docker run -d --name myut -p 80:8080 \
  -e PROXY_HOST=https://your.domain.com \
  -e TARGET_HOST=https://my.utaipei.edu.tw \
  better-myut
```

若前方已有 Nginx / Traefik 等反向代理，僅需將容器埠 (8080) 對接即可。

---

## 進階設定

| 變數 | 預設值 | 說明 |
| --- | --- | --- |
| `PORT` | `8080` | 內部監聽埠號 |
| `TARGET_HOST` | `https://my.utaipei.edu.tw` | 上游校務系統根網址 |
| `PROXY_HOST` | `http://127.0.0.1:8080` | 代理公開網址，用於 HTML 重寫 |

---

## 架構細節

1. **Gin 路由**：`router.Any("/*proxyPath", proxy.ProxyHandler)` 對所有路徑進行攔截。
2. **ProxyHandler**：呼叫 `doProxyRequest` 進行真正的 HTTP 轉發並處理 30x 重定向。
3. **optimizeHTML**：
   - 置換所有指向原站的 URL → 代理本身。
   - 注入 `InjectedCSS` / `InjectedJS` 與 `<meta viewport>`、快取禁用標籤。
   - 移除干擾觸控體驗的 `oncontextmenu`、右鍵鎖定程式碼。
4. **assets/**：利用 Go `embed` 嵌入編譯後產生的二進位，部署更輕鬆。

---

## 貢獻

歡迎提出 Issue、Pull Request 或建議！

---

## 授權 License

本專案採用 **MIT License**，詳見 [LICENSE](LICENSE)。

### 歡迎學校拿去用！