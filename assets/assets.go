package assets

import _ "embed"

//go:embed injected.css
var InjectedCSS string

//go:embed injected.js
var InjectedJS string

//go:embed font/TaipeiSansTCBeta-Light.ttf
var TaipeiSansLight []byte

//go:embed font/TaipeiSansTCBeta-Regular.ttf
var TaipeiSansRegular []byte

//go:embed font/TaipeiSansTCBeta-Bold.ttf
var TaipeiSansBold []byte
