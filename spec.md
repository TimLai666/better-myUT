# 更好的校務系統

## 目標

https://my.utaipei.edu.tw/utaipei/index_sky.html

優化以上網站的CSS，做到響應式設計。

### 技術棧

Go語言

### 功能

在後端處理網站傳來的頁面，優化CSS，並回傳優化後的頁面。
必須能維持登入狀態，就像直接操作圓網站一樣。

### 開發時

開發時如果遇到需要登入的頁面，使用`.env`中的`UTAIPEI_ACCOUNT`和`UTAIPEI_PASSWORD`登入。