// Injected helper JS for better-myUT
(function(){
  function updateSwitch() {
    try {
      const bannerDoc = parent && parent.banner && parent.banner.document;
      const mainDoc   = frames['Main'] && frames['Main'].document;
      if (!bannerDoc || !mainDoc) return;

      const btn = bannerDoc.getElementById('switch');
      if (!btn) return;

      const chk = mainDoc.getElementById('chk');
      if (chk) {
        // 移到畫面外並停用互動
        btn.style.position = 'fixed';
        btn.style.left = '-9999px';
        btn.style.top  = '-9999px';
        btn.style.pointerEvents = 'none';
      } else {
        // 恢復原狀（若按鈕需要顯示時）
        btn.style.position = '';
        btn.style.left = '';
        btn.style.top  = '';
        btn.style.pointerEvents = '';
      }
    } catch(e) {}
  }

  // 初始化一次，之後每 300ms 檢查
  updateSwitch();
  setInterval(updateSwitch, 300);
})();

// ---------------- 響應式 Header 美化 ----------------
(function(){
  const MOBILE_BREAK = 768;

  // 為 banner 文件插入 viewport，避免縮放錯誤
  function ensureViewport(doc){
    if(!doc) return;
    const metaExisting = doc.querySelector('meta[name="viewport"]');
    if(metaExisting){
      metaExisting.setAttribute('content','width=device-width,initial-scale=1');
    }else{
      const meta = doc.createElement('meta');
      meta.name = 'viewport';
      meta.content = 'width=device-width,initial-scale=1';
      doc.head.appendChild(meta);
    }
  }

  // 動態調整 frameset/ frame 寬度
  function adaptFrames(isMobile){
    try {
      const topDoc = window.top.document;

      // 處理 banner frame 寬度
      const bannerFrame = topDoc.getElementById('banner') || topDoc.querySelector('frame[name="banner"]');
      if(bannerFrame){
        bannerFrame.style.width = '100dvw';
        bannerFrame.setAttribute('scrolling','no');
        bannerFrame.setAttribute('marginwidth','0');
        bannerFrame.setAttribute('marginheight','0');
        // 插入 viewport
        try{ ensureViewport(bannerFrame.contentDocument || bannerFrame.contentWindow.document);}catch(e){}
      }

      // 若存在多欄 frameset，行動版改為單欄
      const sets = Array.from(topDoc.getElementsByTagName('frameset'));
      sets.forEach(fs=>{
        if(!fs.dataset.origCols){
          fs.dataset.origCols = fs.getAttribute('cols') || '';
        }
        if(isMobile){
          fs.setAttribute('cols','*');
        }else{
          if(fs.dataset.origCols) fs.setAttribute('cols',fs.dataset.origCols);
        }
      });
    } catch(err) {}
  }

  // 動態調整 banner header 內容
  function adaptHeader(){
    try {
      // 取得 banner 文件（若有）
      const bannerFrame = window.top.banner || window.top.document.getElementById('banner') || null;
      const doc = (bannerFrame && (bannerFrame.contentDocument || bannerFrame.document)) || document;

      const infoSpan = doc.getElementById('info');
      if(!infoSpan) return;

      const tr = infoSpan.closest('tr');
      if(!tr) return;

      const tds = tr.children;
      if(tds.length < 3) return;

      const spacerTD = tds[0];
      const mainTD   = tds[1];
      const serverTD = tds[2];

      const isMobile = window.matchMedia(`(max-width: ${MOBILE_BREAK}px)`).matches;

      if(isMobile){
        spacerTD.style.display = 'none';

        const table = tr.closest('table');
        if(table){
          table.style.width = '100%';
          table.style.tableLayout = 'fixed';
        }

        mainTD.style.display = 'flex';
        mainTD.style.flexWrap = 'wrap';
        mainTD.style.alignItems = 'center';
        mainTD.style.gap = '4px 8px';
        mainTD.style.padding = '8px 0';

        mainTD.querySelectorAll('span, font').forEach(el=>{
          el.style.whiteSpace = 'normal';
          el.style.wordBreak = 'break-word';
        });

        mainTD.querySelectorAll('input.button').forEach(btn=>{
          btn.style.padding = '6px 12px';
          btn.style.fontSize = '12px';
        });

        serverTD.style.display = 'block';
        serverTD.style.textAlign = 'left';
        serverTD.style.fontSize = '12px';
        serverTD.style.paddingTop = '6px';
        serverTD.style.wordBreak = 'break-word';
      }else{
        spacerTD.style.display = '';

        const table = tr.closest('table');
        if(table){
          table.style.width = '';
          table.style.tableLayout = '';
        }

        mainTD.style.display = '';
        mainTD.style.flexWrap = '';
        mainTD.style.alignItems = '';
        mainTD.style.gap = '';
        mainTD.style.padding = '';

        mainTD.querySelectorAll('span, font').forEach(el=>{
          el.style.whiteSpace = '';
          el.style.wordBreak = '';
        });

        mainTD.querySelectorAll('input.button').forEach(btn=>{
          btn.style.padding = '';
          btn.style.fontSize = '';
        });

        serverTD.style.display = '';
        serverTD.style.textAlign = '';
        serverTD.style.fontSize = '';
        serverTD.style.paddingTop = '';
        serverTD.style.wordBreak = '';
      }
    }catch(e){}
  }

  // 統一呼叫
  function adaptAll(){
    const isMobile = window.matchMedia(`(max-width: ${MOBILE_BREAK}px)`).matches;
    adaptFrames(isMobile);
    adaptHeader();
  }

  adaptAll();
  window.addEventListener('resize', adaptAll);

  // 保險機制：每 1 秒確保 banner frame 寬度保持 100dvw
  function fixBannerWidth(){
    try{
      const bannerF = window.top.document.getElementById('banner') || window.top.document.querySelector('frame[name="banner"]');
      if(bannerF){
        bannerF.style.width = '100dvw';
      }
    }catch(e){}
  }
  fixBannerWidth();
  setInterval(fixBannerWidth, 1000);
})();