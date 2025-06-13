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

type ProxyServer struct {
	client     *http.Client
	targetHost string // upstream 目標網站
	publicHost string // 部署後對外的代理伺服器網址
}

func NewProxyServer() *ProxyServer {
	jar, _ := cookiejar.New(nil)
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
		// 跳過某些不應該轉發的 headers
		// if key == "Connection" || key == "Keep-Alive" || key == "Proxy-Authenticate" ||
		// 	key == "Proxy-Authorization" || key == "Te" || key == "Trailers" ||
		// 	key == "Transfer-Encoding" || key == "Upgrade" {
		// 	continue
		// }

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

	// 處理路徑：去掉 /utaipei 前綴
	currentURL := p.targetHost + r.URL.Path
	if r.URL.RawQuery != "" {
		currentURL += "?" + r.URL.RawQuery
	}

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
			// 跳過某些不應該轉發的 headers
			// if key == "Connection" || key == "Keep-Alive" || key == "Proxy-Authenticate" ||
			// 	key == "Proxy-Authorization" || key == "Te" || key == "Trailers" ||
			// 	key == "Transfer-Encoding" || key == "Upgrade" {
			// 	continue
			// }

			for _, value := range values {
				proxyReq.Header.Add(key, value)
			}
		}

		// 設置正確的 Host header
		proxyReq.Host = proxyReq.URL.Host

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

	// 防止頁面自行跳轉到 perchk.jsp
	html = strings.ReplaceAll(html, `top.location="perchk.jsp"`, `top.location="/utaipei/index_sky.html"`)
	html = strings.ReplaceAll(html, `top.location='perchk.jsp'`, `top.location='/utaipei/index_sky.html'`)
	html = strings.ReplaceAll(html, `parent.location="perchk.jsp"`, `parent.location="/utaipei/index_sky.html'`)
	html = strings.ReplaceAll(html, `parent.location='perchk.jsp'`, `parent.location='/utaipei/index_sky.html'`)

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

	// 排除清單：不注入 favorite.jsp
	reqPath := strings.ToLower(c.Request.URL.Path)
	shouldInject := isHTML && !strings.HasSuffix(reqPath, "/favorite.jsp")

	if shouldInject {
		body = p.optimizeHTML(body)
	}

	// 確保後續邏輯知道是否修改過 HTML
	isHTML = shouldInject

	// 複製 headers
	for key, values := range resp.Header {
		// 若我們修改了 HTML 內容，就不要複製 Content-Length
		if isHTML && strings.ToLower(key) == "content-length" {
			continue
		}
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
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

	// 其他所有路徑也交給 proxy（但排除已定義的 /font 路由）
	router.NoRoute(proxy.ProxyHandler)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("啟動伺服器失敗: %v", err)
	}
}
