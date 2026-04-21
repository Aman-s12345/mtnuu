package service

import _ "embed"

// indexHTMLTemplate is the Scalar documentation page, templatized
// with fields from config.Config. It is compiled into the binary
// via go:embed, so consumers of mtnuu never need to ship assets
// alongside their executable.
//
//go:embed index.html.tmpl
var indexHTMLTemplate string