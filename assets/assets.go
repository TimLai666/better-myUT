package assets

import _ "embed"

//go:embed fonts.css
var FontsCSS string

//go:embed base.css
var BaseCSS string

//go:embed buttons.css
var ButtonsCSS string

//go:embed forms.css
var FormsCSS string

//go:embed sidebar.css
var SidebarCSS string

//go:embed modal.css
var ModalCSS string

//go:embed header.css
var HeaderCSS string

//go:embed tables.css
var TablesCSS string

//go:embed injected.js
var InjectedJS string

//go:embed font/TaipeiSansTCBeta-Light.ttf
var TaipeiSansLight []byte

//go:embed font/TaipeiSansTCBeta-Regular.ttf
var TaipeiSansRegular []byte

//go:embed font/TaipeiSansTCBeta-Bold.ttf
var TaipeiSansBold []byte

// CombinedCSS 將所有 CSS 模組組合成一個字串
var CombinedCSS = FontsCSS + "\n\n" + BaseCSS + "\n\n" + ButtonsCSS + "\n\n" + FormsCSS + "\n\n" + SidebarCSS + "\n\n" + ModalCSS + "\n\n" + HeaderCSS + "\n\n" + TablesCSS
