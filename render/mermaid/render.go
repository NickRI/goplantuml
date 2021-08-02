package mermaid

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/jfeliu007/goplantuml/parser"
	"github.com/jfeliu007/goplantuml/render"
)

const extends = `Inheritance`
const implements = `Realization`
const aggregates = `Aggregation`
const aliasOf = `Alias`

type renderer struct {
}

var _ render.Renderer = (*renderer)(nil)

func NewRender() *renderer {
	return &renderer{}
}

func (r *renderer) Render(p *parser.ClassParser) string {
	str := &parser.LineStringBuilder{}
	str.WriteLineWithDepth(0, "classDiagram")

	var packages []string
	for pack := range p.Structure {
		packages = append(packages, pack)
	}
	sort.Strings(packages)
	for _, pack := range packages {
		structures := p.Structure[pack]
		r.renderStructures(p, pack, structures, str)

	}
	if p.RenderingOptions.Aliases {
		r.renderAliases(p, str)
	}
	return str.String()
}

func (r *renderer) renderStructures(p *parser.ClassParser, pack string, structures map[string]*parser.Struct, str *parser.LineStringBuilder) {
	if len(structures) > 0 {
		composition := &parser.LineStringBuilder{}
		extends := &parser.LineStringBuilder{}
		aggregations := &parser.LineStringBuilder{}
		//str.WriteLineWithDepth(0, fmt.Sprintf(`namespace %s {`, pack))

		var names []string
		for name := range structures {
			names = append(names, name)
		}

		sort.Strings(names)

		for _, name := range names {
			structure := structures[name]
			r.renderStructure(p, structure, pack, name, str, composition, extends, aggregations)
		}

		//str.WriteLineWithDepth(0, fmt.Sprintf(`}`))
		if p.RenderingOptions.Compositions {
			str.WriteLineWithDepth(0, composition.String())
		}
		if p.RenderingOptions.Implementations {
			str.WriteLineWithDepth(0, extends.String())
		}
		if p.RenderingOptions.Aggregations {
			str.WriteLineWithDepth(0, aggregations.String())
		}
	}
}

func (r *renderer) renderStructure(p *parser.ClassParser, structure *parser.Struct, pack string, name string, str *parser.LineStringBuilder, composition *parser.LineStringBuilder, extends *parser.LineStringBuilder, aggregations *parser.LineStringBuilder) {
	privateFields := &parser.LineStringBuilder{}
	publicFields := &parser.LineStringBuilder{}
	privateMethods := &parser.LineStringBuilder{}
	publicMethods := &parser.LineStringBuilder{}
	sType := ""
	renderStructureType := structure.Type
	switch structure.Type {
	case "interface":
		sType = "<<interface>>"
		renderStructureType = "class"
	case "class":
		sType = "<<class>>"
	case "alias":
		sType = "<<alias>> "
		renderStructureType = "class"

	}
	str.WriteLineWithDepth(1, fmt.Sprintf(`%s %s { %s`, renderStructureType, r.underscore(pack+"_"+name), sType))
	r.renderStructFields(p, structure, privateFields, publicFields)
	r.renderStructMethods(p, structure, privateMethods, publicMethods)
	r.renderCompositions(p, structure, name, composition)
	r.renderExtends(p, structure, name, extends)
	r.renderAggregations(p, structure, name, aggregations)
	if privateFields.Len() > 0 {
		str.WriteLineWithDepth(0, privateFields.String())
	}
	if publicFields.Len() > 0 {
		str.WriteLineWithDepth(0, publicFields.String())
	}
	if privateMethods.Len() > 0 {
		str.WriteLineWithDepth(0, privateMethods.String())
	}
	if publicMethods.Len() > 0 {
		str.WriteLineWithDepth(0, publicMethods.String())
	}
	str.WriteLineWithDepth(1, fmt.Sprintf(`}`))
}

func (r *renderer) renderAggregations(p *parser.ClassParser, structure *parser.Struct, name string, aggregations *parser.LineStringBuilder) {
	aggregationMap := structure.Aggregations
	if p.RenderingOptions.AggregatePrivateMembers {
		r.updatePrivateAggregations(structure, aggregationMap)
	}
	r.renderAggregationMap(p, aggregationMap, structure, aggregations, name)
}

func (r *renderer) updatePrivateAggregations(structure *parser.Struct, aggregationsMap map[string]struct{}) {
	for agg := range structure.PrivateAggregations {
		aggregationsMap[agg] = struct{}{}
	}
}

func (r *renderer) renderCompositions(p *parser.ClassParser, structure *parser.Struct, name string, composition *parser.LineStringBuilder) {
	var orderedCompositions []string

	for c := range structure.Composition {
		if !strings.Contains(c, ".") {
			c = fmt.Sprintf("%s.%s", p.GetPackageName(c, structure), c)
		}
		composedString := ""
		if p.RenderingOptions.ConnectionLabels {
			composedString = extends
		}
		c = fmt.Sprintf(`%s --|> %s_%s : %s`, r.underscore(c), r.underscore(structure.PackageName), name, composedString)
		orderedCompositions = append(orderedCompositions, c)
	}
	sort.Strings(orderedCompositions)
	for _, c := range orderedCompositions {
		composition.WriteLineWithDepth(0, c)
	}
}

func (r *renderer) underscore(val string) string {
	return strings.ReplaceAll(val, ".", "_")
}

func (r *renderer) renderAggregationMap(p *parser.ClassParser, aggregationMap map[string]struct{}, structure *parser.Struct, aggregations *parser.LineStringBuilder, name string) {
	var orderedAggregations []string
	for a := range aggregationMap {
		orderedAggregations = append(orderedAggregations, a)
	}

	sort.Strings(orderedAggregations)

	for _, a := range orderedAggregations {
		if !strings.Contains(a, ".") {
			a = fmt.Sprintf("%s.%s", p.GetPackageName(a, structure), a)
		}
		aggregationString := ""
		if p.RenderingOptions.ConnectionLabels {
			aggregationString = aggregates
		}
		if p.GetPackageName(a, structure) != parser.BuiltinPackageName {
			aggregations.WriteLineWithDepth(0, fmt.Sprintf(`%s_%s --o %s : %s`, r.underscore(structure.PackageName), name, r.underscore(a), aggregationString))
		}
	}
}

func (r *renderer) renderExtends(p *parser.ClassParser, structure *parser.Struct, name string, extends *parser.LineStringBuilder) {
	var orderedExtends []string
	for c := range structure.Extends {
		if !strings.Contains(c, ".") {
			c = fmt.Sprintf("%s.%s", structure.PackageName, c)
		}
		implementString := ""
		if p.RenderingOptions.ConnectionLabels {
			implementString = implements
		}
		c = fmt.Sprintf(`%s <|.. %s_%s : %s`, r.underscore(c), r.underscore(structure.PackageName), name, implementString)
		orderedExtends = append(orderedExtends, c)
	}
	sort.Strings(orderedExtends)
	for _, c := range orderedExtends {
		extends.WriteLineWithDepth(0, c)
	}
}

func (r *renderer) renderStructMethods(p *parser.ClassParser, structure *parser.Struct, privateMethods *parser.LineStringBuilder, publicMethods *parser.LineStringBuilder) {

	for _, method := range structure.Functions {
		accessModifier := "+"
		if unicode.IsLower(rune(method.Name[0])) {
			if !p.RenderingOptions.PrivateMembers {
				continue
			}

			accessModifier = "-"
		}
		parameterList := make([]string, 0)
		for _, p := range method.Parameters {
			parameterList = append(parameterList, fmt.Sprintf("%s %s", p.Name, r.underscore(p.Type)))
		}
		returnValues := ""
		if len(method.ReturnValues) > 0 {
			if len(method.ReturnValues) == 1 {
				returnValues = r.underscore(method.ReturnValues[0])
			} else {
				returnValues = fmt.Sprintf("(%s)", r.underscore(strings.Join(method.ReturnValues, ", ")))
			}
		}
		if accessModifier == "-" {
			privateMethods.WriteLineWithDepth(2, fmt.Sprintf(`%s%s(%s) %s`, accessModifier, method.Name, strings.Join(parameterList, ", "), returnValues))
		} else {
			publicMethods.WriteLineWithDepth(2, fmt.Sprintf(`%s%s(%s) %s`, accessModifier, method.Name, strings.Join(parameterList, ", "), returnValues))
		}
	}
}

func (r *renderer) renderStructFields(p *parser.ClassParser, structure *parser.Struct, privateFields, publicFields *parser.LineStringBuilder) {
	for _, field := range structure.Fields {
		accessModifier := "+"
		if unicode.IsLower(rune(field.Name[0])) {
			if !p.RenderingOptions.PrivateMembers {
				continue
			}

			accessModifier = "-"
		}
		if accessModifier == "-" {
			privateFields.WriteLineWithDepth(2, fmt.Sprintf(`%s%s %s`, accessModifier, field.Name, strings.ReplaceAll(r.underscore(field.Type), "{}", "")))
		} else {
			publicFields.WriteLineWithDepth(2, fmt.Sprintf(`%s%s %s`, accessModifier, field.Name, strings.ReplaceAll(r.underscore(field.Type), "{}", "")))
		}
	}
}

func (r *renderer) renderAliases(p *parser.ClassParser, str *parser.LineStringBuilder) {
	aliasString := ""
	if p.RenderingOptions.ConnectionLabels {
		aliasString = aliasOf
	}
	orderedAliases := parser.AliasSlice{}
	for _, alias := range p.AllAliases {
		orderedAliases = append(orderedAliases, *alias)
	}
	sort.Sort(orderedAliases)
	for _, alias := range orderedAliases {
		aliasName := alias.Name
		if strings.Count(alias.Name, ".") > 1 {
			split := strings.SplitN(alias.Name, ".", 2)
			if aliasRename, ok := p.AllRenamedStructs[split[0]]; ok {
				renamed := parser.GenerateRenamedStructName(split[1])
				if _, ok := aliasRename[renamed]; ok {
					aliasName = fmt.Sprintf("%s.%s", split[0], renamed)
				}
			}
		}
		str.WriteLineWithDepth(0, fmt.Sprintf(`%s .. %s : %s`, r.underscore(aliasName), r.underscore(alias.AliasOf), aliasString))
	}
}
