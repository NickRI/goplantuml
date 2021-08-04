package plantuml

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/AvraamMavridis/randomcolor"
	"github.com/jfeliu007/goplantuml/parser"
	"github.com/jfeliu007/goplantuml/render"
)

const implements = `"implements"`
const extends = `"extends"`
const aggregates = `"uses"`
const aliasOf = `"alias of"`
const nodeSep = "skinparam nodesep 500"
const ranskSep = "skinparam ranksep 1500"

const aliasComplexNameComment = "'This class was created so that we can correctly have an alias pointing to this name. Since it contains dots that can break namespaces"

type renderer struct {
}

var _ render.Renderer = (*renderer)(nil)

func NewRender() *renderer {
	return &renderer{}
}

func (r *renderer) Render(p *parser.ClassParser) string {
	str := &parser.LineStringBuilder{}
	str.WriteLineWithDepth(0, "@startuml")
	str.WriteLineWithDepth(0, nodeSep)
	str.WriteLineWithDepth(0, ranskSep)
	if p.RenderingOptions.Title != "" {
		str.WriteLineWithDepth(0, fmt.Sprintf(`title %s`, p.RenderingOptions.Title))
	}
	if note := strings.TrimSpace(p.RenderingOptions.Notes); note != "" {
		str.WriteLineWithDepth(0, "legend")
		str.WriteLineWithDepth(0, note)
		str.WriteLineWithDepth(0, "end legend")
	}

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
	if !p.RenderingOptions.Fields {
		str.WriteLineWithDepth(0, "hide fields")
	}
	if !p.RenderingOptions.Methods {
		str.WriteLineWithDepth(0, "hide methods")
	}
	str.WriteLineWithDepth(0, "@enduml")
	return str.String()
}

func (r *renderer) renderStructures(p *parser.ClassParser, pack string, structures map[string]*parser.Struct, str *parser.LineStringBuilder) {
	if len(structures) > 0 {
		composition := &parser.LineStringBuilder{}
		extends := &parser.LineStringBuilder{}
		aggregations := &parser.LineStringBuilder{}
		str.WriteLineWithDepth(0, fmt.Sprintf(`namespace %s {`, pack))

		names := []string{}
		for name := range structures {
			names = append(names, name)
		}

		sort.Strings(names)

		for _, name := range names {
			structure := structures[name]
			r.renderStructure(p, structure, pack, name, str, composition, extends, aggregations)
		}
		var orderedRenamedStructs []string
		for tempName := range p.AllRenamedStructs[pack] {
			orderedRenamedStructs = append(orderedRenamedStructs, tempName)
		}
		sort.Strings(orderedRenamedStructs)
		for _, tempName := range orderedRenamedStructs {
			name := p.AllRenamedStructs[pack][tempName]
			str.WriteLineWithDepth(1, fmt.Sprintf(`class "%s" as %s {`, name, tempName))
			str.WriteLineWithDepth(2, aliasComplexNameComment)
			str.WriteLineWithDepth(1, "}")
		}
		str.WriteLineWithDepth(0, fmt.Sprintf(`}`))
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

func (r *renderer) renderAliases(p *parser.ClassParser, str *parser.LineStringBuilder) {
	var randColor = randomcolor.GetRandomColorInHex()
	var aliasString string
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
		str.WriteLineWithDepth(0, fmt.Sprintf(`"%s" #.[%s]. %s"%s"`, aliasName, randColor, aliasString, alias.AliasOf))
	}
}

func (r *renderer) renderStructure(
	p *parser.ClassParser,
	structure *parser.Struct,
	pack string,
	name string,
	str *parser.LineStringBuilder,
	composition *parser.LineStringBuilder,
	extends *parser.LineStringBuilder,
	aggregations *parser.LineStringBuilder,
) {

	privateFields := &parser.LineStringBuilder{}
	publicFields := &parser.LineStringBuilder{}
	privateMethods := &parser.LineStringBuilder{}
	publicMethods := &parser.LineStringBuilder{}
	sType := ""
	renderStructureType := structure.Type
	switch structure.Type {
	case "class":
		sType = "<< (S,Aquamarine) >>"
	case "alias":
		sType = "<< (T, #FF7700) >> "
		renderStructureType = "class"

	}
	str.WriteLineWithDepth(1, fmt.Sprintf(`%s %s %s {`, renderStructureType, name, sType))
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

func (r *renderer) renderCompositions(p *parser.ClassParser, structure *parser.Struct, name string, composition *parser.LineStringBuilder) {
	var randColor = randomcolor.GetRandomColorInHex()
	var orderedCompositions []string

	for c := range structure.Composition {
		if !strings.Contains(c, ".") {
			c = fmt.Sprintf("%s.%s", p.GetPackageName(c, structure), c)
		}
		composedString := ""
		if p.RenderingOptions.ConnectionLabels {
			composedString = extends
		}
		c = fmt.Sprintf(`"%s" *-[%s]- %s"%s.%s"`, c, randColor, composedString, structure.PackageName, name)
		orderedCompositions = append(orderedCompositions, c)
	}
	sort.Strings(orderedCompositions)
	for _, c := range orderedCompositions {
		composition.WriteLineWithDepth(0, c)
	}
}

func (r *renderer) updatePrivateAggregations(structure *parser.Struct, aggregationsMap map[string]struct{}) {

	for agg := range structure.PrivateAggregations {
		aggregationsMap[agg] = struct{}{}
	}
}

func (r *renderer) renderAggregationMap(p *parser.ClassParser, aggregationMap map[string]struct{}, structure *parser.Struct, aggregations *parser.LineStringBuilder, name string) {
	var randColor = randomcolor.GetRandomColorInHex()
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
			aggregations.WriteLineWithDepth(0, fmt.Sprintf(`"%s.%s"%s o-[%s]- "%s"`, structure.PackageName, name, aggregationString, randColor, a))
		}
	}
}

func (r *renderer) renderExtends(p *parser.ClassParser, structure *parser.Struct, name string, extends *parser.LineStringBuilder) {
	var randColor = randomcolor.GetRandomColorInHex()
	var orderedExtends []string
	for c := range structure.Extends {
		if !strings.Contains(c, ".") {
			c = fmt.Sprintf("%s.%s", structure.PackageName, c)
		}
		implementString := ""
		if p.RenderingOptions.ConnectionLabels {
			implementString = implements
		}
		c = fmt.Sprintf(`"%s" <|-[%s]- %s"%s.%s"`, c, randColor, implementString, structure.PackageName, name)
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
			parameterList = append(parameterList, fmt.Sprintf("%s %s", p.Name, p.Type))
		}
		returnValues := ""
		if len(method.ReturnValues) > 0 {
			if len(method.ReturnValues) == 1 {
				returnValues = method.ReturnValues[0]
			} else {
				returnValues = fmt.Sprintf("(%s)", strings.Join(method.ReturnValues, ", "))
			}
		}
		if accessModifier == "-" {
			privateMethods.WriteLineWithDepth(2, fmt.Sprintf(`%s %s(%s) %s`, accessModifier, method.Name, strings.Join(parameterList, ", "), returnValues))
		} else {
			publicMethods.WriteLineWithDepth(2, fmt.Sprintf(`%s %s(%s) %s`, accessModifier, method.Name, strings.Join(parameterList, ", "), returnValues))
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
			privateFields.WriteLineWithDepth(2, fmt.Sprintf(`%s %s %s`, accessModifier, field.Name, field.Type))
		} else {
			publicFields.WriteLineWithDepth(2, fmt.Sprintf(`%s %s %s`, accessModifier, field.Name, field.Type))
		}
	}
}
