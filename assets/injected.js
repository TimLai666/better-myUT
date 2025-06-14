window.addEventListener('load', () => {
    // 頁面完全載入
    const banner = frames['banner'];
    const main = frames['Main'];
    if (banner) {
        console.log('🚀 通過 load 事件獲取到 banner:', banner);

        const logoDiv = banner.document.querySelector('.schoolLogo');
        if (logoDiv) {
            const smallLogo = banner.document.createElement('img');
            smallLogo.src = "/assets/img/icon.png";
            smallLogo.alt = "small logo";
            smallLogo.id = "smallLogo";
            logoDiv.appendChild(smallLogo);

            const logoImg = banner.document.createElement('img');
            logoImg.src = "/utaipei/pics/logo.png";
            logoImg.alt = "logo";
            logoImg.id = "logo";
            logoDiv.appendChild(logoImg);

            console.log('✅ Logo 已添加');
        } else {
            console.log('❌ 找不到 .schoolLogo 元素');
        }
    }

    insertFooter(main);

    // 添加側邊欄搜尋功能
    setTimeout(initSearch, 1000);
});


function insertFooter(frame) {
    // 為頁面 body 添加底部間距，避免內容被 footer 覆蓋
    frame.document.body.style.paddingBottom = '130px';

    const footer = frame.document.createElement('div');
    footer.id = 'customFooter';
    footer.style.cssText = `
        position: fixed;
        bottom: 0;
        left: 0;
        right: 0;
        min-height: 8%;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        row-gap: 0px;
        gap: 0px;
        background-color: #f8f9fa;
        padding: 2px 10px;
        font-size: 14px;
        line-height: 1.5;
        color: #6c757d;
        box-shadow: 0 -2px 5px rgba(0,0,0,0.1);
        z-index: 1000;
        word-break: break-all;
    `;
    footer.innerHTML = `<p style="margin:0;">此介面改良版本由 <a href="https://github.com/TimLai666" target="_blank" style="color: #007bff; text-decoration: none;">TimLai666</a> 提供，歡迎學校採用。然由於此系統使用了過時已遭淘汰的 Frameset 技術，建議建置全新系統。</p>
    <p style="margin:0;">此專案的開源儲存庫：<a href="https://github.com/TimLai666/better-myUT" target="_blank" style="color: #007bff; text-decoration: none;">點我</a></p>`;
    frame.document.body.appendChild(footer);
}



function initSearch() {
    console.log('🔄 嘗試初始化搜尋功能...');

    const leftMenu = window.frames['Lmenu'];
    if (!leftMenu || !leftMenu.document) {
        console.log('❌ 找不到左側選單框架，2秒後重試...');
        setTimeout(initSearch, 2000);
        return;
    }

    const treeDiv = leftMenu.document.getElementById('m_tree');
    if (!treeDiv || treeDiv.innerHTML.trim().length === 0) {
        console.log('❌ 選單尚未載入完成，2秒後重試...');
        setTimeout(initSearch, 2000);
        return;
    }

    console.log('✅ 開始初始化搜尋功能');
    createSearchUI(leftMenu);
}

// 創建搜尋介面
function createSearchInterface(doc, treeDiv) {
    // 創建搜尋容器
    const searchContainer = doc.createElement('div');
    searchContainer.style.cssText = `
        position: sticky;
        top: 0;
        background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        padding: 15px;
        z-index: 1000;
        box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        width: 243px;
        box-sizing: border-box;
    `;

    // 創建搜尋輸入框容器
    const inputContainer = doc.createElement('div');
    inputContainer.style.cssText = 'position: relative;';

    // 創建搜尋輸入框
    const searchInput = doc.createElement('input');
    searchInput.type = 'text';
    searchInput.id = 'searchInput';
    searchInput.placeholder = '🔍 搜尋功能...';
    searchInput.style.cssText = `
        width: 100%;
        padding: 12px 40px 12px 15px;
        border: none;
        border-radius: 25px;
        font-size: 14px;
        outline: none;
        box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        background: white;
        transition: box-shadow 0.3s ease;
        box-sizing: border-box;
    `;

    // 創建統計信息
    const searchStats = doc.createElement('div');
    searchStats.id = 'searchStats';
    searchStats.style.cssText = `
        color: white;
        font-size: 12px;
        margin-top: 8px;
        text-align: center;
        display: none;
    `;

    // 創建搜尋結果容器
    const searchResults = doc.createElement('div');
    searchResults.id = 'searchResults';
    searchResults.style.cssText = `
        display: none;
        background: #ffffff;
        border: 1px solid #e9ecef;
        border-radius: 8px;
        min-height: 100px;
        max-height: 500px;
        overflow-y: auto;
        margin-top: 10px;
        box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        z-index: 1001;
        position: relative;
        width: 100%;
    `;

    // 組裝搜尋容器
    inputContainer.appendChild(searchInput);
    searchContainer.appendChild(inputContainer);
    searchContainer.appendChild(searchStats);
    searchContainer.appendChild(searchResults);

    // 插入到選單前面
    treeDiv.parentNode.insertBefore(searchContainer, treeDiv);

    console.log('✅ 搜尋介面創建完成');

    // 測試搜尋框是否可以獲得焦點
    setTimeout(() => {
        console.log('🔍 測試搜尋框焦點...');
        searchInput.focus();
        console.log('🔍 搜尋框是否有焦點:', doc.activeElement === searchInput);
        console.log('🔍 搜尋框元素:', searchInput);
    }, 1000);

    return { searchInput, searchStats, searchResults };
}

function createSearchUI(menuFrame) {
    const doc = menuFrame.document;
    const treeDiv = doc.getElementById('m_tree');

    // 創建一個全局的 menuItems 陣列
    let menuItems = [];

    // 創建搜尋介面
    const { searchInput, searchStats, searchResults } = createSearchInterface(doc, treeDiv);

    // 發送所有 HTML 到後端處理，完成後綁定事件
    sendAllHTMLToBackend(doc, (items) => {
        menuItems = items;
        console.log(`📋 後端處理完成，共 ${menuItems.length} 個選單項目`);

        // 現在綁定搜尋事件
        setupSearchEvents(doc, menuItems, treeDiv, searchInput, searchStats, searchResults);
        console.log('✅ 搜尋功能已啟用');
    });
}

// 發送所有 HTML 到後端處理
function sendAllHTMLToBackend(doc, callback) {
    console.log('🔄 發送所有 HTML 到後端處理...');

    // 收集所有功能項目和分類項目
    const functionDivs = doc.querySelectorAll('div[onclick*="of_display"]');
    const categorySpans = doc.querySelectorAll('span.shand');

    console.log(`⚡ 找到功能項目: ${functionDivs.length} 個`);
    console.log(`📁 找到分類項目: ${categorySpans.length} 個`);

    const allItems = [];
    let completedRequests = 0;
    const totalRequests = 2; // 功能項目 + 分類項目

    // 處理功能項目
    if (functionDivs.length > 0) {
        const functionElements = Array.from(functionDivs).map(div => ({
            html: div.outerHTML
        }));

        fetch('/api/parse-html', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                htmlElements: functionElements,
                type: 'function'
            })
        })
            .then(response => response.json())
            .then(data => {
                console.log(`✅ 後端功能解析成功: ${data.items.length} 個項目`);

                // 添加功能項目到結果中
                data.items.forEach((item, index) => {
                    if (index < functionDivs.length) {
                        allItems.push({
                            text: item.text,
                            code: item.code,
                            element: functionDivs[index],
                            type: item.type
                        });
                    }
                });

                completedRequests++;
                if (completedRequests === totalRequests) {
                    callback(allItems);
                }
            })
            .catch(error => {
                console.error('❌ 後端功能處理失敗:', error);
                completedRequests++;
                if (completedRequests === totalRequests) {
                    callback(allItems);
                }
            });
    } else {
        completedRequests++;
    }

    // 處理分類項目
    if (categorySpans.length > 0) {
        const categoryElements = Array.from(categorySpans).map(span => ({
            html: span.outerHTML
        }));

        fetch('/api/parse-html', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                htmlElements: categoryElements,
                type: 'category'
            })
        })
            .then(response => response.json())
            .then(data => {
                console.log(`✅ 後端分類解析成功: ${data.items.length} 個項目`);

                // 添加分類項目到結果中
                data.items.forEach((item, index) => {
                    if (index < categorySpans.length && item.text && !item.text.includes('編輯我的最愛')) {
                        allItems.push({
                            text: item.text,
                            element: categorySpans[index],
                            type: item.type
                        });
                    }
                });

                completedRequests++;
                if (completedRequests === totalRequests) {
                    callback(allItems);
                }
            })
            .catch(error => {
                console.error('❌ 後端分類處理失敗:', error);
                completedRequests++;
                if (completedRequests === totalRequests) {
                    callback(allItems);
                }
            });
    } else {
        completedRequests++;
    }
}

function setupSearchEvents(doc, menuItems, originalTree, searchInput, searchStats, searchResults) {
    let searchTimeout;

    console.log(`🔍 搜尋事件已綁定，可搜尋項目數量: ${menuItems.length}`);

    // 搜尋輸入事件
    searchInput.addEventListener('input', (e) => {
        const query = e.target.value;
        console.log(`🔎 搜尋查詢: "${query}" (長度: ${query.length})`);

        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            if (query.length > 0) {
                console.log(`🔍 開始搜尋: "${query}"，在 ${menuItems.length} 個項目中搜尋`);
                showSearchResults(query, menuItems, searchResults, searchStats, originalTree, doc);
            } else {
                console.log('🔍 清空搜尋結果');
                hideSearchResults(searchResults, searchStats, originalTree);
            }
        }, 200);
    });

    // 添加額外的事件監聽器來調試
    searchInput.addEventListener('keyup', (e) => {
        console.log(`⌨️ keyup 事件: "${e.target.value}"`);
    });

    searchInput.addEventListener('change', (e) => {
        console.log(`🔄 change 事件: "${e.target.value}"`);
    });



    // 鍵盤事件
    searchInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            const firstResult = searchResults.querySelector('.search-item');
            if (firstResult) {
                firstResult.click();
            }
        }
        if (e.key === 'Escape') {
            searchInput.value = '';
            hideSearchResults(searchResults, searchStats, originalTree);
        }
    });

    // 懸浮效果
    searchInput.addEventListener('focus', () => {
        searchInput.style.boxShadow = '0 4px 15px rgba(0,0,0,0.2)';
    });

    searchInput.addEventListener('blur', () => {
        searchInput.style.boxShadow = '0 2px 8px rgba(0,0,0,0.1)';
    });
}

function showSearchResults(query, menuItems, resultsDiv, statsDiv, originalTree, doc) {
    console.log(`🔍 搜尋函數被調用，查詢: "${query}"，項目數量: ${menuItems.length}`);

    // 先檢查 menuItems 的內容
    if (menuItems.length > 0) {
        console.log('📋 前幾個項目示例:', menuItems.slice(0, 3).map(item => ({
            text: item.text,
            type: item.type,
            code: item.code
        })));
    }

    const matches = menuItems.filter(item =>
        item.type === 'function' && item.text.toLowerCase().includes(query.toLowerCase())
    );

    console.log(`✅ 找到 ${matches.length} 個匹配項目`);

    // 清空結果容器
    resultsDiv.innerHTML = '';

    if (matches.length === 0) {
        const noResults = doc.createElement('div');
        noResults.style.cssText = `
            padding: 30px 20px;
            text-align: center;
            color: #6c757d;
            height: 100px;
            min-height: 100px;
            max-height: 100px;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
        `;
        noResults.innerHTML = `
            <div style="font-size: 48px; margin-bottom: 15px;">🔍</div>
            <div style="font-size: 16px; font-weight: 500;">找不到匹配的功能</div>
        `;
        resultsDiv.appendChild(noResults);
    } else {
        matches.forEach((item, index) => {
            console.log(`🔍 處理搜尋項目 ${index + 1}:`, item.text);

            // 創建搜尋項目元素
            const searchItem = doc.createElement('div');
            searchItem.className = 'search-item';
            searchItem.style.cssText = `
                padding: 15px 20px;
                border-bottom: 1px solid #e9ecef;
                cursor: pointer;
                transition: background-color 0.2s ease;
                display: flex;
                align-items: center;
                gap: 12px;
                min-height: 50px;
                color: #333333 !important;
                font-size: 14px;
                line-height: 1.4;
                background: white;
            `;

            // 直接設置文字內容，不做任何處理
            const iconSpan = doc.createElement('span');
            iconSpan.style.fontSize = '16px';
            iconSpan.textContent = item.type === 'category' ? '📁' : '⚡';

            const textSpan = doc.createElement('span');
            textSpan.style.flex = '1';
            textSpan.style.color = '#333333';
            textSpan.style.fontWeight = '500';
            textSpan.textContent = item.text + (item.code ? ` (${item.code})` : '');

            const typeSpan = doc.createElement('span');
            typeSpan.style.fontSize = '12px';
            typeSpan.style.color = '#6c757d';
            typeSpan.textContent = item.type === 'category' ? '分類' : '功能';

            searchItem.appendChild(iconSpan);
            searchItem.appendChild(textSpan);
            searchItem.appendChild(typeSpan);

            console.log(`🔍 項目內容:`, searchItem.textContent);

            // 添加懸浮效果
            searchItem.addEventListener('mouseover', () => {
                searchItem.style.backgroundColor = '#e3f2fd';
            });
            searchItem.addEventListener('mouseout', () => {
                searchItem.style.backgroundColor = 'transparent';
            });

            // 添加點擊事件
            searchItem.addEventListener('click', () => {
                console.log('點擊搜尋項目:', item.text, item.code || '');
                item.element.click();
            });

            resultsDiv.appendChild(searchItem);
        });
    }

    statsDiv.textContent = `找到 ${matches.length} 個結果`;
    statsDiv.style.display = 'block';
    resultsDiv.style.display = 'block';
    originalTree.style.display = 'none';

    console.log('🎯 搜尋結果已顯示:', {
        statsVisible: statsDiv.style.display,
        resultsVisible: resultsDiv.style.display,
        originalTreeHidden: originalTree.style.display,
        matchesCount: matches.length,
        resultsContainer: resultsDiv,
        originalTreeElement: originalTree
    });
}

function hideSearchResults(resultsDiv, statsDiv, originalTree) {
    resultsDiv.style.display = 'none';
    statsDiv.style.display = 'none';
    originalTree.style.display = 'block';
}