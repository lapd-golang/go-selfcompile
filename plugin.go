package selfcompile

import (
	"errors"
	"fmt"
	"io"
)

const tmplPlugin = `// Generated by go-selfcompile.
package %s

`

const tmplImport = `import _ "%s"
`

var errMissingImport = errors.New("missing import string for plugin")

type plugin struct {
	Package string
	Imports []string
}

func (p plugin) WriteTo(w io.Writer) (int64, error) {
	pkg := p.Package
	if pkg == "" {
		pkg = "main"
	}
	if len(p.Imports) == 0 {
		return 0, errMissingImport
	}
	n, err := fmt.Fprintf(w, tmplPlugin, p.Package)
	if err != nil {
		return int64(n), err
	}
	for _, imp := range p.Imports {
		nn, err := fmt.Fprintf(w, tmplImport, imp)
		n += nn
		if err != nil {
			return int64(n), err
		}
	}
	return int64(n), nil
}