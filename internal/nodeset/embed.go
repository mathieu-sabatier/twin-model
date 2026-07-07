package nodeset

import (
	"embed"
	"io/fs"
)

//go:embed specs/*.NodeSet2.xml
var specsFS embed.FS

// openSpec opens a vendored NodeSet2 by base filename.
func openSpec(file string) (fs.File, error) {
	return specsFS.Open("specs/" + file)
}
