package main

import (
	"better-myUT/assets"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// 輔助函數
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type ProxyServer struct {
	client     *http.Client
	targetHost string // upstream 目標網站
	publicHost string // 部署後對外的代理伺服器網址
}

func NewProxyServer() *ProxyServer {
	// 創建一個更寬鬆的cookie jar設置
	jar, err := cookiejar.New(&cookiejar.Options{
		PublicSuffixList: nil, // 允許更寬鬆的cookie處理
	})
	if err != nil {
		log.Printf("警告：創建cookie jar失敗: %v", err)
		jar, _ = cookiejar.New(nil)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// 從環境變數讀取目標與公開主機
	target := os.Getenv("TARGET_HOST")
	public := os.Getenv("PROXY_HOST")

	// 預設值
	if target == "" {
		target = "https://my.utaipei.edu.tw"
	}
	if public == "" {
		public = "http://127.0.0.1:8080"
	}

	log.Printf("代理伺服器設置 - 目標: %s, 公開: %s", target, public)

	return &ProxyServer{
		client:     client,
		targetHost: target,
		publicHost: public,
	}
}

func (p *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 記錄請求資訊
	log.Printf("收到請求: %s %s", r.Method, r.URL.String())

	// 處理代理請求，自動跟隨重定向
	finalResp, finalBody, err := p.doProxyRequest(r)
	if err != nil {
		log.Printf("代理請求失敗: %v", err)
		http.Error(w, "代理請求失敗", http.StatusBadGateway)
		return
	}
	defer finalResp.Body.Close()

	log.Printf("最終回應: %d %s", finalResp.StatusCode, finalResp.Status)

	// 檢查是否為 HTML 內容，需要進行優化
	contentType := finalResp.Header.Get("Content-Type")
	isHTML := strings.Contains(strings.ToLower(contentType), "text/html")

	// 如果是 HTML 內容，進行 CSS 優化
	if isHTML {
		log.Printf("優化 HTML 內容")
		finalBody = p.optimizeHTML(finalBody)
	}

	// 複製回應 headers，但排除某些不應該轉發的 headers
	for key, values := range finalResp.Header {
		// 如果我們修改了 HTML 內容，就不要複製 Content-Length header
		if isHTML && strings.ToLower(key) == "content-length" {
			continue
		}

		// 跳過原始的快取相關 headers，我們會設置自己的
		if strings.ToLower(key) == "cache-control" || strings.ToLower(key) == "pragma" ||
			strings.ToLower(key) == "expires" || strings.ToLower(key) == "etag" ||
			strings.ToLower(key) == "last-modified" {
			continue
		}

		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 添加禁用快取的 headers
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
	w.Header().Set("X-Cache-Control", "no-cache")

	log.Printf("已設置禁用快取的 headers")

	// 返回 200 OK 而不是重定向狀態碼
	w.WriteHeader(http.StatusOK)
	w.Write(finalBody)

	log.Printf("完成代理請求，返回優化內容")
}

// 新增函數：處理代理請求並自動跟隨重定向
func (p *ProxyServer) doProxyRequest(r *http.Request) (*http.Response, []byte, error) {
	maxRedirects := 100

	// 使用完整路徑，不去掉前綴
	path := r.URL.Path
	currentURL := p.targetHost + path
	if r.URL.RawQuery != "" {
		currentURL += "?" + r.URL.RawQuery
	}

	log.Printf("URL路徑處理: %s -> %s", r.URL.Path, currentURL)

	// 保存原始請求的 body（如果有的話）
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("讀取請求 body 失敗: %v", err)
		}
		r.Body.Close()
	}

	for i := 0; i < maxRedirects; i++ {
		log.Printf("代理到 (第%d次): %s", i+1, currentURL)

		// 重建請求 body
		var requestBody io.Reader
		if len(bodyBytes) > 0 {
			requestBody = strings.NewReader(string(bodyBytes))
		}

		// 創建代理請求
		proxyReq, err := http.NewRequest(r.Method, currentURL, requestBody)
		if err != nil {
			return nil, nil, fmt.Errorf("創建代理請求失敗: %v", err)
		}

		// 複製原始請求的 headers
		for key, values := range r.Header {
			// 跳過某些可能會造成問題的headers
			lowerKey := strings.ToLower(key)
			if lowerKey == "host" {
				continue // Host header 已經在下面單獨設置
			}

			// 特別處理Cookie header
			if lowerKey == "cookie" {
				for _, value := range values {
					// 記錄原始cookie
					log.Printf("🍪 轉發Cookie: %s", value)
					proxyReq.Header.Add(key, value)
				}
				continue
			}

			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		// 設置正確的 Host header
		proxyReq.Host = proxyReq.URL.Host

		// 確保重要的認證相關headers正確設置
		if proxyReq.Header.Get("Referer") == "" && r.Header.Get("Referer") != "" {
			// 將Referer中的代理地址替換為目標地址
			referer := r.Header.Get("Referer")
			referer = strings.ReplaceAll(referer, p.publicHost, p.targetHost)
			proxyReq.Header.Set("Referer", referer)
		}

		// 🔐 一律確保所有請求都有完整的認證和瀏覽器headers

		// 確保User-Agent（如果沒有則設置預設值）
		if proxyReq.Header.Get("User-Agent") == "" {
			if r.Header.Get("User-Agent") != "" {
				proxyReq.Header.Set("User-Agent", r.Header.Get("User-Agent"))
			} else {
				// 設置預設的瀏覽器User-Agent
				proxyReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			}
		}

		// 確保Accept header（根據請求類型設置）
		if proxyReq.Header.Get("Accept") == "" {
			if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				// Ajax請求
				proxyReq.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
			} else {
				// 一般HTML請求
				proxyReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
			}
		}

		// 確保Accept-Language
		if proxyReq.Header.Get("Accept-Language") == "" {
			proxyReq.Header.Set("Accept-Language", "zh-TW,zh;q=0.9,en;q=0.8")
		}

		// 確保Accept-Encoding
		if proxyReq.Header.Get("Accept-Encoding") == "" {
			proxyReq.Header.Set("Accept-Encoding", "gzip, deflate")
		}

		// 一律設置防快取headers（確保認證狀態即時更新）
		proxyReq.Header.Set("Cache-Control", "no-cache")
		proxyReq.Header.Set("Pragma", "no-cache")

		// 確保Connection header
		if proxyReq.Header.Get("Connection") == "" {
			proxyReq.Header.Set("Connection", "keep-alive")
		}

		// 確保Upgrade-Insecure-Requests
		if proxyReq.Header.Get("Upgrade-Insecure-Requests") == "" && r.Method == "GET" {
			proxyReq.Header.Set("Upgrade-Insecure-Requests", "1")
		}

		// 對於Ajax請求，確保X-Requested-With
		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			proxyReq.Header.Set("X-Requested-With", "XMLHttpRequest")
		}

		// 記錄特殊認證檢查請求
		if strings.Contains(strings.ToLower(currentURL), "perchk.jsp") ||
			strings.Contains(strings.ToLower(currentURL), "check") ||
			strings.Contains(strings.ToLower(currentURL), "auth") {
			log.Printf("🔐 認證檢查請求: %s", currentURL)
		}

		// 設置Origin header（對於CORS很重要）
		if origin := r.Header.Get("Origin"); origin != "" {
			// 將Origin中的代理地址替換為目標地址
			origin = strings.ReplaceAll(origin, p.publicHost, p.targetHost)
			proxyReq.Header.Set("Origin", origin)
		} else if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			// 對於修改性請求，如果沒有Origin則設置一個
			proxyReq.Header.Set("Origin", p.targetHost)
		}

		// 創建不跟隨重定向的 client
		tempClient := &http.Client{
			Jar: p.client.Jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Timeout: p.client.Timeout,
		}

		// 執行請求
		resp, err := tempClient.Do(proxyReq)
		if err != nil {
			return nil, nil, fmt.Errorf("執行代理請求失敗: %v", err)
		}

		// 讀取回應內容
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, nil, fmt.Errorf("讀取回應失敗: %v", err)
		}

		// 檢查是否是重定向
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			if location == "" {
				log.Printf("重定向回應缺少 Location header，直接返回該回應")
				// 如果沒有 Location header，直接返回這個回應
				return resp, body, nil
			}

			log.Printf("檢測到重定向: %d -> %s", resp.StatusCode, location)

			// 處理相對 URL
			if strings.HasPrefix(location, "/") {
				currentURL = p.targetHost + location
			} else if strings.HasPrefix(location, "http") {
				// 若導向 localhost，改寫成目標主機路徑
				if strings.HasPrefix(location, "http://localhost") || strings.HasPrefix(location, "https://localhost") {
					if parsed, err := url.Parse(location); err == nil {
						currentURL = p.targetHost + parsed.Path
						if parsed.RawQuery != "" {
							currentURL += "?" + parsed.RawQuery
						}
					} else {
						currentURL = p.targetHost
					}
				} else {
					currentURL = location
				}
			} else {
				// 相對路徑，需要基於當前 URL 構建
				baseURL := currentURL
				if lastSlash := strings.LastIndex(baseURL, "/"); lastSlash > 8 { // 8 是 "https://" 的長度
					baseURL = baseURL[:lastSlash+1]
				}
				currentURL = baseURL + location
			}

			resp.Body.Close()

			// 對於重定向，通常改為 GET 請求（除非是 307/308）
			if resp.StatusCode != 307 && resp.StatusCode != 308 {
				r.Method = "GET"
				bodyBytes = nil // 清空 body
			}

			continue
		}

		// 不是重定向，返回結果
		return resp, body, nil
	}

	return nil, nil, fmt.Errorf("超過最大重定向次數 (%d)", maxRedirects)
}

func (p *ProxyServer) optimizeHTML(html []byte) []byte {
	htmlStr := string(html)

	// URL 替換：將目標網站的 URL 替換成代理伺服器的 URL
	htmlStr = p.replaceTargetURLs(htmlStr, "")

	// 處理 frameset：將 frameset 轉換為直接內容插入
	// htmlStr = p.convertFramesetToContent(htmlStr)

	// 移除右鍵選單禁用
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu="CancelEvent (event, 'oncontextmenu')"`, "")
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu='CancelEvent (event, "oncontextmenu")'`, "")
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu="return false"`, "")
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu='return false'`, "")

	// 移除可能的右鍵禁用 JavaScript
	htmlStr = regexp.MustCompile(`(?i)oncontextmenu\s*=\s*["'][^"']*["']`).ReplaceAllString(htmlStr, "")

	// 讀取外部 injectedCSS 資料
	responsiveCSS := "\n<style>\n" + assets.InjectedCSS + "\n</style>"

	// 如為 frameset 頁（頂層），再注入 JavaScript
	jsInjection := ""
	if strings.Contains(strings.ToLower(htmlStr), "<frameset") {
		jsInjection = "\n<script>\n" + assets.InjectedJS + "\n</script>"
	}

	// 檢查並插入 viewport
	viewportMeta := `<meta name="viewport" content="width=device-width,initial-scale=1">`

	if !strings.Contains(strings.ToLower(htmlStr), "<meta name=\"viewport\"") {
		htmlStr = strings.Replace(htmlStr, "<head>", "<head>"+viewportMeta, 1)
	}

	// 添加禁用快取的 meta 標籤
	noCacheMetaTags := `
<meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate">
<meta http-equiv="Pragma" content="no-cache">
<meta http-equiv="Expires" content="0">
<meta name="robots" content="noindex, nofollow, noarchive, nosnippet, noimageindex">
`

	// 在 </head> 之前插入 CSS 和 meta 標籤
	headEndRegex := regexp.MustCompile(`(?i)</head>`)
	if headEndRegex.MatchString(htmlStr) {
		htmlStr = headEndRegex.ReplaceAllString(htmlStr, noCacheMetaTags+responsiveCSS+jsInjection+"</head>")
	} else {
		// 如果沒有 head 標籤，在 body 開始後插入
		bodyStartRegex := regexp.MustCompile(`(?i)<body[^>]*>`)
		if bodyStartRegex.MatchString(htmlStr) {
			htmlStr = bodyStartRegex.ReplaceAllStringFunc(htmlStr, func(match string) string {
				return match + noCacheMetaTags + responsiveCSS + jsInjection
			})
		} else {
			// 如果既沒有 <head> 也沒有 <body>，最後採用最保險方案：直接把 CSS 及 meta 標籤放到最前面
			htmlStr = noCacheMetaTags + viewportMeta + responsiveCSS + jsInjection + htmlStr
		}
	}

	// 為表格添加 data-label 屬性以支援響應式設計
	htmlStr = p.addTableDataLabels(htmlStr)

	return []byte(htmlStr)
}

func (p *ProxyServer) addTableDataLabels(html string) string {
	// 這是一個簡化的實現，實際使用中可能需要更複雜的 HTML 解析
	// 為表格的 td 添加 data-label 屬性

	// 找到所有表格並為其添加響應式支援
	tableRegex := regexp.MustCompile(`(?s)<table[^>]*>(.*?)</table>`)

	return tableRegex.ReplaceAllStringFunc(html, func(tableHTML string) string {
		// 提取表頭
		theadRegex := regexp.MustCompile(`(?s)<thead[^>]*>(.*?)</thead>`)
		theadMatch := theadRegex.FindStringSubmatch(tableHTML)

		if len(theadMatch) > 1 {
			// 提取表頭中的 th 標籤
			thRegex := regexp.MustCompile(`<th[^>]*>(.*?)</th>`)
			thMatches := thRegex.FindAllStringSubmatch(theadMatch[1], -1)

			var headers []string
			for _, match := range thMatches {
				// 去除 HTML 標籤，只保留文字內容
				headerText := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(match[1], "")
				headers = append(headers, strings.TrimSpace(headerText))
			}

			// 為 tbody 中的 td 添加 data-label
			if len(headers) > 0 {
				tbodyRegex := regexp.MustCompile(`(?s)<tbody[^>]*>(.*?)</tbody>`)
				tableHTML = tbodyRegex.ReplaceAllStringFunc(tableHTML, func(tbodyHTML string) string {
					trRegex := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
					return trRegex.ReplaceAllStringFunc(tbodyHTML, func(trHTML string) string {
						tdRegex := regexp.MustCompile(`<td([^>]*)>(.*?)</td>`)
						tdIndex := 0
						return tdRegex.ReplaceAllStringFunc(trHTML, func(tdHTML string) string {
							if tdIndex < len(headers) {
								// 在現有的屬性中添加 data-label
								tdMatch := tdRegex.FindStringSubmatch(tdHTML)
								if len(tdMatch) > 2 {
									attrs := tdMatch[1]
									content := tdMatch[2]
									newTd := fmt.Sprintf(`<td%s data-label="%s">%s</td>`, attrs, headers[tdIndex], content)
									tdIndex++
									return newTd
								}
							}
							tdIndex++
							return tdHTML
						})
					})
				})
			}
		}

		return tableHTML
	})
}

func (p *ProxyServer) replaceTargetURLs(html string, basePath string) string {
	// 取得代理伺服器對外網址
	proxyHost := p.publicHost
	if proxyHost == "" {
		proxyHost = "http://127.0.0.1:8080"
	}

	// 替換絕對 URL
	html = strings.ReplaceAll(html, "https://my.utaipei.edu.tw", proxyHost)
	html = strings.ReplaceAll(html, "http://my.utaipei.edu.tw", proxyHost)

	// 將可能寫成 localhost 的 URL 一併導向代理（避免撈取本機 80 port）
	html = strings.ReplaceAll(html, "https://localhost", proxyHost+"/utaipei")
	html = strings.ReplaceAll(html, "http://localhost", proxyHost+"/utaipei")
	html = strings.ReplaceAll(html, "//localhost", proxyHost+"/utaipei")

	// 處理各種形式的 JavaScript 重定向
	html = strings.ReplaceAll(html, `window.location.href="https://my.utaipei.edu.tw`, `window.location.href="`+proxyHost)
	html = strings.ReplaceAll(html, `window.location="https://my.utaipei.edu.tw`, `window.location="`+proxyHost)
	html = strings.ReplaceAll(html, `location.href="https://my.utaipei.edu.tw`, `location.href="`+proxyHost)
	html = strings.ReplaceAll(html, `location="https://my.utaipei.edu.tw`, `location="`+proxyHost)
	html = strings.ReplaceAll(html, `document.location="https://my.utaipei.edu.tw`, `document.location="`+proxyHost)
	html = strings.ReplaceAll(html, `document.location.href="https://my.utaipei.edu.tw`, `document.location.href="`+proxyHost)

	// 處理單引號的情況
	html = strings.ReplaceAll(html, `window.location.href='https://my.utaipei.edu.tw`, `window.location.href='`+proxyHost)
	html = strings.ReplaceAll(html, `window.location='https://my.utaipei.edu.tw`, `window.location='`+proxyHost)
	html = strings.ReplaceAll(html, `location.href='https://my.utaipei.edu.tw`, `location.href='`+proxyHost)
	html = strings.ReplaceAll(html, `location='https://my.utaipei.edu.tw`, `location='`+proxyHost)

	// 處理 meta refresh 重定向
	html = regexp.MustCompile(`<meta[^>]*http-equiv="refresh"[^>]*content="[^"]*url=https://my\.utaipei\.edu\.tw([^"]*)"[^>]*>`).ReplaceAllStringFunc(html, func(match string) string {
		return strings.ReplaceAll(match, "https://my.utaipei.edu.tw", proxyHost)
	})

	// 處理表單 action
	html = strings.ReplaceAll(html, `action="https://my.utaipei.edu.tw`, `action="`+proxyHost)
	html = strings.ReplaceAll(html, `action='https://my.utaipei.edu.tw`, `action='`+proxyHost)

	// 處理 iframe src
	html = strings.ReplaceAll(html, `src="https://my.utaipei.edu.tw`, `src="`+proxyHost)
	html = strings.ReplaceAll(html, `src='https://my.utaipei.edu.tw`, `src='`+proxyHost)

	// 處理 link href
	html = strings.ReplaceAll(html, `href="https://my.utaipei.edu.tw`, `href="`+proxyHost)
	html = strings.ReplaceAll(html, `href='https://my.utaipei.edu.tw`, `href='`+proxyHost)

	// 移除或修改任何可能導致重定向到原站的腳本
	// 檢查是否有任何 top.location 或 parent.location 的重定向
	html = strings.ReplaceAll(html, `top.location="https://my.utaipei.edu.tw`, `top.location="`+proxyHost)
	html = strings.ReplaceAll(html, `parent.location="https://my.utaipei.edu.tw`, `parent.location="`+proxyHost)
	html = strings.ReplaceAll(html, `top.location='https://my.utaipei.edu.tw`, `top.location='`+proxyHost)
	html = strings.ReplaceAll(html, `parent.location='https://my.utaipei.edu.tw`, `parent.location='`+proxyHost)
	return html
}

// Gin 版代理處理器
func (p *ProxyServer) ProxyHandler(c *gin.Context) {
	// 若為根路徑則導向入口頁
	if c.Request.URL.Path == "/" {
		c.Redirect(http.StatusFound, "/utaipei/index_sky.html")
		return
	}

	// 記錄請求
	log.Printf("收到請求: %s %s", c.Request.Method, c.Request.URL.String())

	// 詳細記錄認證相關的headers（用於除錯）
	if cookies := c.Request.Header.Get("Cookie"); cookies != "" {
		log.Printf("Cookie: %s", cookies)
	}
	if userAgent := c.Request.Header.Get("User-Agent"); userAgent != "" {
		log.Printf("User-Agent: %s", userAgent)
	}
	if xRequestedWith := c.Request.Header.Get("X-Requested-With"); xRequestedWith != "" {
		log.Printf("X-Requested-With: %s", xRequestedWith)
	}
	if referer := c.Request.Header.Get("Referer"); referer != "" {
		log.Printf("Referer: %s", referer)
	}
	if origin := c.Request.Header.Get("Origin"); origin != "" {
		log.Printf("Origin: %s", origin)
	}

	// 使用既有邏輯執行代理請求，包含自動重定向
	resp, body, err := p.doProxyRequest(c.Request)
	if err != nil {
		log.Printf("代理請求失敗: %v", err)
		c.String(http.StatusBadGateway, "代理請求失敗")
		return
	}
	defer resp.Body.Close()

	// 檢查是否為 HTML，且不在排除清單再進行注入
	contentType := resp.Header.Get("Content-Type")
	isHTML := strings.Contains(strings.ToLower(contentType), "text/html")

	// 檢查是否為二進制文件（字體、圖片等）
	isBinaryFile := false
	lowerContentType := strings.ToLower(contentType)
	lowerPath := strings.ToLower(c.Request.URL.Path)

	// 檢查Content-Type與路徑是否匹配
	if (strings.HasSuffix(lowerPath, ".ttf") ||
		strings.HasSuffix(lowerPath, ".woff") ||
		strings.HasSuffix(lowerPath, ".woff2") ||
		strings.HasSuffix(lowerPath, ".otf") ||
		strings.HasSuffix(lowerPath, ".eot")) &&
		!strings.Contains(lowerContentType, "font") {
		log.Printf("⚠️  字體文件Content-Type不匹配! 路徑: %s, Content-Type: %s", c.Request.URL.Path, contentType)
		log.Printf("回應前100字元: %s", string(body[:min(100, len(body))]))

		// 檢查是否實際是字體文件內容
		if len(body) > 4 {
			// WOFF文件以"wOFF"開頭
			if string(body[:4]) == "wOFF" {
				log.Printf("檢測到WOFF字體文件，修正Content-Type")
				isBinaryFile = true
				if strings.HasSuffix(lowerPath, ".woff2") {
					contentType = "font/woff2"
				} else {
					contentType = "font/woff"
				}
			}
			// TTF文件通常以特定字節序列開頭
			if len(body) > 8 && (body[0] == 0x00 && body[1] == 0x01 && body[2] == 0x00 && body[3] == 0x00) {
				log.Printf("檢測到TTF字體文件，修正Content-Type")
				isBinaryFile = true
				contentType = "font/ttf"
			}
		}
	}

	if strings.Contains(lowerContentType, "font") ||
		strings.Contains(lowerContentType, "image") ||
		strings.Contains(lowerContentType, "video") ||
		strings.Contains(lowerContentType, "audio") ||
		strings.Contains(lowerContentType, "application/octet-stream") ||
		strings.Contains(lowerContentType, "application/pdf") ||
		strings.Contains(lowerContentType, "application/font") ||
		strings.Contains(lowerContentType, "application/x-font") ||
		strings.HasSuffix(lowerPath, ".ttf") ||
		strings.HasSuffix(lowerPath, ".otf") ||
		strings.HasSuffix(lowerPath, ".woff") ||
		strings.HasSuffix(lowerPath, ".woff2") ||
		strings.HasSuffix(lowerPath, ".eot") ||
		strings.HasSuffix(lowerPath, ".svg") ||
		strings.HasSuffix(lowerPath, ".png") ||
		strings.HasSuffix(lowerPath, ".jpg") ||
		strings.HasSuffix(lowerPath, ".jpeg") ||
		strings.HasSuffix(lowerPath, ".gif") ||
		strings.HasSuffix(lowerPath, ".ico") ||
		strings.HasSuffix(lowerPath, ".webp") ||
		strings.HasSuffix(lowerPath, ".css") ||
		strings.HasSuffix(lowerPath, ".js") ||
		strings.Contains(lowerPath, "/font") {
		isBinaryFile = true
		log.Printf("檢測到二進制/靜態文件: %s (Content-Type: %s)", c.Request.URL.Path, contentType)
	}

	// 排除清單：不注入 favorite.jsp、API路徑和二進制文件
	reqPath := strings.ToLower(c.Request.URL.Path)
	shouldInject := isHTML && !isBinaryFile &&
		!strings.HasSuffix(reqPath, "/favorite.jsp") &&
		!strings.Contains(reqPath, "_api.jsp") &&
		!strings.Contains(reqPath, "/api/") &&
		!strings.Contains(reqPath, "api.jsp")

	if shouldInject {
		body = p.optimizeHTML(body)
		log.Printf("已對HTML內容進行優化")
	} else if isBinaryFile {
		log.Printf("跳過二進制文件的HTML優化")
	}

	// 特別記錄API和權限檢查回應內容（用於除錯登入狀態）
	if strings.Contains(reqPath, "favorite_api.jsp") || strings.Contains(reqPath, "api") ||
		strings.Contains(reqPath, "perchk.jsp") || strings.Contains(reqPath, "check") {
		log.Printf("🔐 認證相關回應 (%s): 狀態=%d, 內容=%s",
			c.Request.URL.Path, resp.StatusCode, string(body[:min(500, len(body))]))
	}

	// 確保後續邏輯知道是否修改過 HTML
	isHTML = shouldInject

	// 複製 headers
	for key, values := range resp.Header {
		// 若我們修改了 HTML 內容，就不要複製 Content-Length
		if isHTML && strings.ToLower(key) == "content-length" {
			continue
		}

		// 處理Set-Cookie headers - 需要將domain修改為代理domain
		if strings.ToLower(key) == "set-cookie" {
			for _, value := range values {
				// 將cookie中的domain從原站改為代理站
				modifiedCookie := p.transformSetCookie(value)
				c.Writer.Header().Add(key, modifiedCookie)
			}
			continue
		}

		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	// 修正字體文件的Content-Type
	if isBinaryFile {
		if strings.HasSuffix(lowerPath, ".ttf") {
			c.Writer.Header().Set("Content-Type", "font/ttf")
		} else if strings.HasSuffix(lowerPath, ".woff") {
			c.Writer.Header().Set("Content-Type", "font/woff")
		} else if strings.HasSuffix(lowerPath, ".woff2") {
			c.Writer.Header().Set("Content-Type", "font/woff2")
		} else if strings.HasSuffix(lowerPath, ".eot") {
			c.Writer.Header().Set("Content-Type", "application/vnd.ms-fontobject")
		} else if strings.HasSuffix(lowerPath, ".otf") {
			c.Writer.Header().Set("Content-Type", "font/otf")
		}

		// 如果我們之前修正了contentType，使用修正後的值
		if contentType != resp.Header.Get("Content-Type") {
			c.Writer.Header().Set("Content-Type", contentType)
		}

		// 確保二進制文件不會被快取禁用影響
		c.Writer.Header().Del("Cache-Control")
		c.Writer.Header().Del("Pragma")
		c.Writer.Header().Del("Expires")
		c.Writer.Header().Set("Cache-Control", "public, max-age=31536000")
	}

	// 添加CORS headers以支援Ajax請求
	origin := c.Request.Header.Get("Origin")
	if origin != "" && !isBinaryFile {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Accept, Origin, Cache-Control, Pragma, Cookie, Referer")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, Set-Cookie, Location")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 預檢請求快取1天
	}

	// 處理OPTIONS預檢請求
	if c.Request.Method == "OPTIONS" {
		c.Status(http.StatusOK)
		return
	}

	// 如為 HTML，添加我們自己的 Content-Length
	if isHTML {
		c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	}

	// 直接沿用遠端狀態碼
	c.Status(resp.StatusCode)

	// 回傳 body
	c.Writer.Write(body)
	log.Printf("完成代理請求 (gin)")
}

// 轉換Set-Cookie header，使其適用於代理域名
func (p *ProxyServer) transformSetCookie(cookieValue string) string {
	// 解析代理主機的域名
	proxyURL, err := url.Parse(p.publicHost)
	if err != nil {
		log.Printf("警告：無法解析代理主機URL: %v", err)
		return cookieValue
	}

	proxyDomain := proxyURL.Hostname()

	// 保留原始cookie值用於比較
	originalCookie := cookieValue

	// 對於本地測試，採用更保守的處理方式
	if proxyDomain == "127.0.0.1" || proxyDomain == "localhost" {
		// 只移除不相容的domain設定，保留其他屬性
		modifiedCookie := cookieValue

		// 檢查是否有domain設定需要移除
		if strings.Contains(strings.ToLower(cookieValue), "domain=") {
			// 只移除與目標網站相關的domain，保留認證相關的設定
			domainRegex := regexp.MustCompile(`(?i);\s*domain=([^;]*\.)?utaipei\.edu\.tw`)
			modifiedCookie = domainRegex.ReplaceAllString(modifiedCookie, "")
			log.Printf("🔧 移除domain限制: %s -> %s", cookieValue, modifiedCookie)
		}

		// 對於HTTP代理，移除secure屬性
		if !strings.HasPrefix(p.publicHost, "https://") {
			modifiedCookie = regexp.MustCompile(`(?i);\s*secure\s*`).ReplaceAllString(modifiedCookie, "")
		}

		// 確保session cookie有正確的path
		if !strings.Contains(strings.ToLower(modifiedCookie), "path=") {
			modifiedCookie += "; Path=/"
		}

		log.Printf("Cookie轉換 (localhost): %s -> %s", originalCookie, modifiedCookie)
		return modifiedCookie
	}

	// 對於生產環境的處理
	modifiedCookie := cookieValue

	// 替換domain為代理domain
	domainRegex := regexp.MustCompile(`(?i);\s*domain=[^;]*`)
	modifiedCookie = domainRegex.ReplaceAllString(modifiedCookie, "; Domain="+proxyDomain)

	// 如果是HTTPS代理就保留secure，否則移除
	if !strings.HasPrefix(p.publicHost, "https://") {
		modifiedCookie = regexp.MustCompile(`(?i);\s*secure\s*`).ReplaceAllString(modifiedCookie, "")
	}

	// 確保cookie對所有路徑有效
	if !strings.Contains(strings.ToLower(modifiedCookie), "path=") {
		modifiedCookie += "; Path=/"
	}

	log.Printf("Cookie轉換 (production): %s -> %s", originalCookie, modifiedCookie)
	return modifiedCookie
}

func main() {
	// 載入環境變數
	if err := godotenv.Load(".env"); err != nil {
		log.Println("警告：未找到 .env 檔案，將使用系統環境變數")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	proxy := NewProxyServer()
	if proxy.targetHost == "" {
		proxy.targetHost = "https://my.utaipei.edu.tw"
	}

	log.Printf("啟動 gin 代理伺服器於端口 %s", port)
	log.Printf("目標主機: %s", proxy.targetHost)

	router := gin.Default()

	// 添加全面的認證和調試中間件
	router.Use(func(c *gin.Context) {
		// 記錄所有請求的認證狀態
		cookies := c.Request.Header.Get("Cookie")
		userAgent := c.Request.Header.Get("User-Agent")
		referer := c.Request.Header.Get("Referer")

		// 為所有請求記錄基本認證信息
		log.Printf("📨 請求: %s %s | Cookie: %s | UA: %s",
			c.Request.Method,
			c.Request.URL.Path,
			func() string {
				if cookies != "" {
					return "有(" + fmt.Sprintf("%d字元", len(cookies)) + ")"
				}
				return "無"
			}(),
			func() string {
				if userAgent != "" {
					return userAgent[:min(50, len(userAgent))] + "..."
				}
				return "無"
			}())

		// 特別記錄權限檢查請求的完整cookie
		if strings.Contains(c.Request.URL.Path, "perchk.jsp") ||
			strings.Contains(c.Request.URL.Path, "check") {
			log.Printf("🚨 權限檢查: %s", c.Request.URL.String())
			log.Printf("🍪 完整Cookie: %s", cookies)
			log.Printf("🔗 Referer: %s", referer)
		}

		c.Next()
	})

	// 字型檔案路由
	router.GET("/font/TaipeiSansTCBeta-Light.ttf", func(c *gin.Context) {
		c.Header("Content-Type", "font/ttf")
		c.Header("Cache-Control", "public, max-age=31536000") // 1年快取
		c.Data(http.StatusOK, "font/ttf", assets.TaipeiSansLight)
	})

	router.GET("/font/TaipeiSansTCBeta-Regular.ttf", func(c *gin.Context) {
		c.Header("Content-Type", "font/ttf")
		c.Header("Cache-Control", "public, max-age=31536000")
		c.Data(http.StatusOK, "font/ttf", assets.TaipeiSansRegular)
	})

	router.GET("/font/TaipeiSansTCBeta-Bold.ttf", func(c *gin.Context) {
		c.Header("Content-Type", "font/ttf")
		c.Header("Cache-Control", "public, max-age=31536000")
		c.Data(http.StatusOK, "font/ttf", assets.TaipeiSansBold)
	})

	// 根路徑處理
	router.GET("/", proxy.ProxyHandler)

	// utaipei 路徑下的所有請求交給 proxy
	router.Any("/utaipei/*proxyPath", proxy.ProxyHandler)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("啟動伺服器失敗: %v", err)
	}
}
