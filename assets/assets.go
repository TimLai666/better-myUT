package assets

import _ "embed"

//go:embed injected.css
var InjectedCSS string

//go:embed injected.js
var InjectedJS string
