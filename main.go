package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type ProxyServer struct {
	client     *http.Client
	targetHost string
}

func NewProxyServer() *ProxyServer {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	return &ProxyServer{
		client:     client,
		targetHost: os.Getenv("TARGET_HOST"),
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
	isHTML := strings.Contains(contentType, "text/html")

	// 如果是 HTML 內容，進行 CSS 優化
	if isHTML {
		log.Printf("優化 HTML 內容")
		finalBody = p.optimizeHTML(finalBody)
	}

	// 複製回應 headers，但排除某些不應該轉發的 headers
	for key, values := range finalResp.Header {
		// 跳過某些不應該轉發的 headers
		if key == "Connection" || key == "Keep-Alive" || key == "Proxy-Authenticate" ||
			key == "Proxy-Authorization" || key == "Te" || key == "Trailers" ||
			key == "Transfer-Encoding" || key == "Upgrade" {
			continue
		}

		// 如果我們修改了 HTML 內容，就不要複製 Content-Length header
		if isHTML && strings.ToLower(key) == "content-length" {
			continue
		}

		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 返回 200 OK 而不是重定向狀態碼
	w.WriteHeader(http.StatusOK)
	w.Write(finalBody)

	log.Printf("完成代理請求，返回優化內容")
}

// 新增函數：處理代理請求並自動跟隨重定向
func (p *ProxyServer) doProxyRequest(r *http.Request) (*http.Response, []byte, error) {
	maxRedirects := 10
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
			if key == "Connection" || key == "Keep-Alive" || key == "Proxy-Authenticate" ||
				key == "Proxy-Authorization" || key == "Te" || key == "Trailers" ||
				key == "Transfer-Encoding" || key == "Upgrade" {
				continue
			}

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
				currentURL = location
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
	htmlStr = p.replaceTargetURLs(htmlStr)

	// 移除右鍵選單禁用
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu="CancelEvent (event, 'oncontextmenu')"`, "")
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu='CancelEvent (event, "oncontextmenu")'`, "")
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu="return false"`, "")
	htmlStr = strings.ReplaceAll(htmlStr, `oncontextmenu='return false'`, "")

	// 移除可能的右鍵禁用 JavaScript
	htmlStr = regexp.MustCompile(`(?i)oncontextmenu\s*=\s*["'][^"']*["']`).ReplaceAllString(htmlStr, "")

	// 添加響應式 CSS
	responsiveCSS := `
<style>
/* 保守的響應式優化 - 只針對手機端做最小調整 */

/* 確保右鍵選單可用 */
html, body, * {
    -webkit-user-select: text !important;
    -moz-user-select: text !important;
    -ms-user-select: text !important;
    user-select: text !important;
    pointer-events: auto !important;
}

/* 手機版專用 - 只在小螢幕時啟用 */
@media screen and (max-width: 480px) {
    /* 確保頁面不會水平捲動 */
    body, html {
        overflow-x: auto;
        max-width: 100%;
    }
    
    /* Frameset 響應式調整 */
    frameset {
        width: 100% !important;
        height: 100% !important;
    }
    
    /* 調整 frame 的高度比例，讓頂部 banner 更小 */
    frameset[rows*="103"] {
        rows: "80,*" !important;
    }
    
    /* 讓固定寬度的元素變為響應式 */
    table[width] {
        width: 100% !important;
        max-width: 100% !important;
    }
    
    td[width] {
        width: auto !important;
        max-width: 100% !important;
    }
    
    /* 讓圖片響應式 */
    img {
        max-width: 100% !important;
        height: auto !important;
    }
    
    /* 表單元素響應式 */
    input[type="text"], input[type="password"], select, textarea {
        max-width: 100% !important;
        box-sizing: border-box;
        font-size: 16px !important; /* 防止iOS縮放 */
    }
    
    /* 按鈕稍微調整 */
    input[type="button"], input[type="submit"], button {
        min-width: auto;
        padding: 10px 15px;
        font-size: 14px;
        margin: 5px 2px;
        touch-action: manipulation; /* 改善觸控體驗 */
    }
    
    /* iframe 響應式 */
    iframe {
        max-width: 100% !important;
        border: none;
    }
    
    /* 處理可能過寬的內容 */
    div, span, p {
        word-wrap: break-word;
        overflow-wrap: break-word;
        hyphens: auto;
    }
    
    /* 字體稍微調整以便閱讀 */
    body, td, th, div, span, p {
        font-size: 14px !important;
        line-height: 1.5 !important;
    }
    
    /* 表格在小螢幕下的調整 */
    table {
        font-size: 13px;
        border-collapse: collapse;
    }
    
    /* 表格單元格調整 */
    td, th {
        padding: 8px 4px !important;
        vertical-align: top;
    }
    
    /* 確保內容不會被截斷 */
    * {
        max-width: 100%;
        box-sizing: border-box;
    }
    
    /* 改善連結的觸控體驗 */
    a {
        min-height: 44px;
        display: inline-block;
        padding: 8px;
        margin: 2px;
    }
    
    /* 選單和導航優化 */
    .menu, .nav, ul, ol {
        list-style: none;
        padding: 0;
        margin: 0;
    }
    
    /* 隱藏可能不必要的空白區域 */
    td:empty, div:empty {
        display: none;
    }
}

/* 中等螢幕調整 (平板) */
@media screen and (min-width: 481px) and (max-width: 768px) {
    /* Frameset 在平板上的調整 */
    frameset[rows*="103"] {
        rows: "90,*" !important;
    }
    
    img {
        max-width: 100% !important;
        height: auto !important;
    }
    
    table[width] {
        max-width: 100% !important;
    }
    
    iframe {
        max-width: 100% !important;
    }
    
    /* 表單元素在平板上的調整 */
    input[type="text"], input[type="password"], select, textarea {
        font-size: 15px;
        padding: 8px;
    }
    
    /* 按鈕在平板上的調整 */
    input[type="button"], input[type="submit"], button {
        padding: 9px 14px;
        font-size: 14px;
    }
}

/* 大螢幕優化 */
@media screen and (min-width: 769px) {
    /* 確保在大螢幕上正常顯示 */
    frameset {
        width: 100%;
        height: 100%;
    }
}

/* 橫向模式優化 */
@media screen and (max-width: 768px) and (orientation: landscape) {
    /* 橫向模式下調整 frameset 高度 */
    frameset[rows*="103"], frameset[rows*="80"], frameset[rows*="90"] {
        rows: "60,*" !important;
    }
    
    /* 減少按鈕間距 */
    input[type="button"], input[type="submit"], button {
        margin: 2px 1px;
        padding: 6px 10px;
    }
}

/* 打印優化 */
@media print {
    * {
        overflow: visible !important;
    }
    
    frameset, frame {
        display: none !important;
    }
    
    /* 打印時顯示 noframes 內容 */
    noframes {
        display: block !important;
    }
}

/* 高對比度模式支援 */
@media (prefers-contrast: high) {
    input[type="button"], input[type="submit"], button {
        border: 2px solid;
    }
    
    a {
        text-decoration: underline;
    }
}

/* 減少動畫偏好支援 */
@media (prefers-reduced-motion: reduce) {
    * {
        animation-duration: 0.01ms !important;
        animation-iteration-count: 1 !important;
        transition-duration: 0.01ms !important;
    }
}
</style>
`

	// 在 </head> 之前插入 CSS
	headEndRegex := regexp.MustCompile(`(?i)</head>`)
	if headEndRegex.MatchString(htmlStr) {
		htmlStr = headEndRegex.ReplaceAllString(htmlStr, responsiveCSS+"\n</head>")
	} else {
		// 如果沒有 head 標籤，在 body 開始後插入
		bodyStartRegex := regexp.MustCompile(`(?i)<body[^>]*>`)
		htmlStr = bodyStartRegex.ReplaceAllStringFunc(htmlStr, func(match string) string {
			return match + responsiveCSS
		})
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

func (p *ProxyServer) replaceTargetURLs(html string) string {
	// 獲取代理伺服器的地址
	proxyHost := "http://127.0.0.1:8080"

	// 替換絕對 URL
	html = strings.ReplaceAll(html, "https://my.utaipei.edu.tw", proxyHost)
	html = strings.ReplaceAll(html, "http://my.utaipei.edu.tw", proxyHost)

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

// 根路徑重定向處理器
func redirectRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		log.Printf("根路徑重定向: %s -> /utaipei/index_sky.html", r.URL.String())
		http.Redirect(w, r, "http://127.0.0.1:8080/utaipei/index_sky.html", http.StatusFound)
		return
	}
	// 如果不是根路徑，返回 404
	http.NotFound(w, r)
}

func main() {
	// 載入環境變數
	if err := godotenv.Load(); err != nil {
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

	log.Printf("啟動代理伺服器於端口 %s", port)
	log.Printf("目標主機: %s", proxy.targetHost)
	log.Println("CSS 響應式優化已啟用")

	// 設置路由
	http.HandleFunc("/", redirectRoot) // 根路徑重定向
	http.Handle("/utaipei/", proxy)    // 代理所有 utaipei 路徑
	http.Handle("/favicon.ico", proxy) // 處理 favicon
	http.Handle("/index_", proxy)      // 處理 index_ 開頭的文件

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
