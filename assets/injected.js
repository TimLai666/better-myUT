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