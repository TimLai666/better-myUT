window.addEventListener('load', () => {
    // 頁面完全載入
    const banner = frames['banner'];
    if (banner) {
        console.log('🚀 通過 load 事件獲取到 banner:', banner);
        
        const logoDiv = banner.document.querySelector('.schoolLogo');
        if (logoDiv) {
            const logoImg = banner.document.createElement('img');
            logoImg.src = "/utaipei/pics/logo.png";
            logoImg.alt = "logo";
            logoDiv.appendChild(logoImg);
            
            console.log('✅ Logo 已添加');
        } else {
            console.log('❌ 找不到 .schoolLogo 元素');
        }
    }
});