package native

import "embed"

//go:embed app-icon.svg
var Logo []byte

//go:embed app-icon.png
var Icon []byte

//go:embed icons/*.svg icons/*LICENSE
var Icons embed.FS
