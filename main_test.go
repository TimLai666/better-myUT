package main

import (
	"strings"
	"testing"
)

func TestOptimizeHTML(t *testing.T) {
	proxy := &ProxyServer{}

	// 測試基本 HTML 優化
	originalHTML := `
<!DOCTYPE html>
<html>
<head>
    <title>測試頁面</title>
</head>
<body>
    <table>
        <thead>
            <tr>
                <th>姓名</th>
                <th>學號</th>
                <th>成績</th>
            </tr>
        </thead>
        <tbody>
            <tr>
                <td>張三</td>
                <td>123456</td>
                <td>90</td>
            </tr>
        </tbody>
    </table>
</body>
</html>`

	optimizedHTML := string(proxy.optimizeHTML([]byte(originalHTML)))

	// 檢查是否添加了響應式 CSS
	if !strings.Contains(optimizedHTML, "響應式設計優化") {
		t.Error("未找到響應式 CSS")
	}

	// 檢查是否添加了 data-label 屬性
	if !strings.Contains(optimizedHTML, "data-label") {
		t.Error("未添加 data-label 屬性")
	}

	// 檢查 CSS 是否正確插入
	if !strings.Contains(optimizedHTML, "@media screen and (max-width: 768px)") {
		t.Error("未找到手機版媒體查詢")
	}
}

func TestAddTableDataLabels(t *testing.T) {
	proxy := &ProxyServer{}

	tableHTML := `
<table>
    <thead>
        <tr>
            <th>姓名</th>
            <th>學號</th>
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>張三</td>
            <td>123456</td>
        </tr>
    </tbody>
</table>`

	result := proxy.addTableDataLabels(tableHTML)

	// 檢查是否正確添加了 data-label
	if !strings.Contains(result, `data-label="姓名"`) {
		t.Error("未正確添加姓名的 data-label")
	}

	if !strings.Contains(result, `data-label="學號"`) {
		t.Error("未正確添加學號的 data-label")
	}
}

func TestNewProxyServer(t *testing.T) {
	// 使用構造函數創建代理伺服器
	proxy := NewProxyServer()

	if proxy.client == nil {
		t.Error("HTTP 客戶端未初始化")
	}

	if proxy.client.Jar == nil {
		t.Error("Cookie jar 未初始化")
	}
}
