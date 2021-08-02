package render

import "github.com/jfeliu007/goplantuml/parser"

type Renderer interface {
	Render(parser *parser.ClassParser) string
}
