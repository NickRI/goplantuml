/*
Package parser generates PlantUml http://plantuml.com/ Class diagrams for your golang projects
The main Structure is the ClassParser which you can generate by calling the NewClassDiagram(dir)
function.

Pass the directory where the .go files are and the parser will analyze the code and build a Structure
containing the information it needs to Render the class diagram.

call the Render() function and this will return a string with the class diagram.

See github.com/jfeliu007/goplantuml/cmd/goplantuml/main.go for a command that uses this functions and outputs the text to
the console.

*/
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

const tab = "    "
const BuiltinPackageName = "builtin"

// LineStringBuilder extends the strings.Builder and adds functionality to build a string with tabs and
// adding new lines
type LineStringBuilder struct {
	strings.Builder
}

// WriteLineWithDepth will write the given text with added tabs at the beginning into the string builder.
func (lsb *LineStringBuilder) WriteLineWithDepth(depth int, str string) {
	lsb.WriteString(strings.Repeat(tab, depth))
	lsb.WriteString(str)
	lsb.WriteString("\n")
}

// ClassDiagramOptions will provide a way for callers of the NewClassDiagramFs() function to pass all the necessary arguments.
type ClassDiagramOptions struct {
	FileSystem         afero.Fs
	Directories        []string
	IgnoredDirectories []string
	RenderingOptions   map[RenderingOption]interface{}
	Recursive          bool
}

// RenderingOptions will allow the class parser to optionally enebale or disable the things to render.
type RenderingOptions struct {
	Title                   string
	Notes                   string
	ModuleBase              string
	Aggregations            bool
	Fields                  bool
	Methods                 bool
	Compositions            bool
	Implementations         bool
	Aliases                 bool
	ConnectionLabels        bool
	AggregatePrivateMembers bool
	PrivateMembers          bool
}

const (
	// RenderAggregations is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render aggregations
	RenderAggregations RenderingOption = iota

	// RenderCompositions is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render compositions
	RenderCompositions

	// RenderImplementations is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render implementations
	RenderImplementations

	// RenderAliases is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render aliases
	RenderAliases

	// RenderFields is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render fields
	RenderFields

	// RenderMethods is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render methods
	RenderMethods

	// RenderConnectionLabels is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will set the parser to render the connection labels
	RenderConnectionLabels

	// RenderTitle is the options for the Title of the diagram. The value of this will be rendered as a title unless empty
	RenderTitle

	// RenderNotes contains a list of notes to be rendered in the class diagram
	RenderNotes

	// AggregatePrivateMembers is to be used in the SetRenderingOptions argument as the key to the map, when value is true, it will connect aggregations with private members
	AggregatePrivateMembers

	// RenderPrivateMembers is used if private members (fields, methods) should be rendered
	RenderPrivateMembers
)

// RenderingOption is an alias for an it so it is easier to use it as options in a map (see SetRenderingOptions(map[RenderingOption]bool) error)
type RenderingOption int

// ClassParser contains the Structure of the parsed files. The Structure is a map of package_names that contains
// a map of structure_names -> Structs
type ClassParser struct {
	RenderingOptions   *RenderingOptions
	Structure          map[string]map[string]*Struct
	CurrentPackageName string
	AllInterfaces      map[string]struct{}
	AllStructs         map[string]struct{}
	AllImports         map[string]string
	AllAliases         map[string]*Alias
	AllRenamedStructs  map[string]map[string]string
}

// NewClassDiagramWithOptions returns a new classParser with which can Render the class diagram of
// files in the given directory passed in the ClassDiargamOptions. This will also alow for different types of FileSystems
// Passed since it is part of the ClassDiagramOptions as well.
func NewClassDiagramWithOptions(options *ClassDiagramOptions) (*ClassParser, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	classParser := &ClassParser{
		RenderingOptions: &RenderingOptions{
			ModuleBase:       path.Base(cwd),
			Aggregations:     false,
			Fields:           true,
			Methods:          true,
			Compositions:     true,
			Implementations:  true,
			Aliases:          true,
			ConnectionLabels: false,
			Title:            "",
			Notes:            "",
		},
		Structure:         make(map[string]map[string]*Struct),
		AllInterfaces:     make(map[string]struct{}),
		AllStructs:        make(map[string]struct{}),
		AllImports:        make(map[string]string),
		AllAliases:        make(map[string]*Alias),
		AllRenamedStructs: make(map[string]map[string]string),
	}
	ignoreDirectoryMap := map[string]struct{}{}
	for _, dir := range options.IgnoredDirectories {
		ignoreDirectoryMap[dir] = struct{}{}
	}
	for _, directoryPath := range options.Directories {
		if options.Recursive {
			err := afero.Walk(options.FileSystem, directoryPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" {
						return filepath.SkipDir
					}
					if _, ok := ignoreDirectoryMap[path]; ok {
						return filepath.SkipDir
					}
					err := classParser.parseDirectory(path)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			err := classParser.parseDirectory(directoryPath)
			if err != nil {
				return nil, err
			}
		}
	}

	for s := range classParser.AllStructs {
		st := classParser.getStruct(s)
		if st != nil {
			for i := range classParser.AllInterfaces {
				inter := classParser.getStruct(i)
				if st.ImplementsInterface(inter) {
					st.AddToExtends(i)
				}
			}
		}
	}
	err = classParser.SetRenderingOptions(options.RenderingOptions)
	if err != nil {
		return nil, err
	}
	return classParser, nil
}

// NewClassDiagram returns a new classParser with which can Render the class diagram of
// files in the given directory
func NewClassDiagram(directoryPaths []string, ignoreDirectories []string, recursive bool) (*ClassParser, error) {
	options := &ClassDiagramOptions{
		Directories:        directoryPaths,
		IgnoredDirectories: ignoreDirectories,
		Recursive:          recursive,
		RenderingOptions:   map[RenderingOption]interface{}{},
		FileSystem:         afero.NewOsFs(),
	}
	return NewClassDiagramWithOptions(options)
}

// parse the given ast.Package into the ClassParser Structure
func (p *ClassParser) parsePackage(node ast.Node, base string) {
	pack := node.(*ast.Package)
	p.CurrentPackageName = base + "." + pack.Name
	_, ok := p.Structure[p.CurrentPackageName]
	if !ok {
		p.Structure[p.CurrentPackageName] = make(map[string]*Struct)
	}
	var sortedFiles []string
	for fileName := range pack.Files {
		sortedFiles = append(sortedFiles, fileName)
	}
	sort.Strings(sortedFiles)
	for _, fileName := range sortedFiles {

		if !strings.HasSuffix(fileName, "_test.go") {
			f := pack.Files[fileName]
			for _, d := range f.Imports {
				p.parseImports(d)
			}
			for _, d := range f.Decls {
				p.parseFileDeclarations(d)
			}
		}
	}
}

func (p *ClassParser) parseImports(impt *ast.ImportSpec) {
	clean, _ := strconv.Unquote(impt.Path.Value)
	if impt.Name != nil {
		p.AllImports[impt.Name.Name] = strings.ReplaceAll(clean, "/", ".")
	} else {
		chunks := strings.Split(clean, "/")
		p.AllImports[chunks[len(chunks)-1]] = strings.Join(chunks, ".")
	}
}

func (p *ClassParser) parseDirectory(directoryPath string) error {
	fs := token.NewFileSet()

	found := strings.LastIndex(directoryPath, p.RenderingOptions.ModuleBase)
	base := strings.Split(directoryPath[found:], "/")
	result, err := parser.ParseDir(fs, directoryPath, nil, 0)
	if err != nil {
		return err
	}
	for _, v := range result {
		p.parsePackage(v, strings.Join(base[:len(base)-1], "."))
	}
	return nil
}

// parse the given declaration looking for classes, interfaces, or member functions
func (p *ClassParser) parseFileDeclarations(node ast.Decl) {
	switch decl := node.(type) {
	case *ast.GenDecl:
		p.handleGenDecl(decl)
	case *ast.FuncDecl:
		p.handleFuncDecl(decl)
	}
}

func (p *ClassParser) handleFuncDecl(decl *ast.FuncDecl) {

	if decl.Recv != nil {
		if decl.Recv.List == nil {
			return
		}

		// Only get in when the function is defined for a Structure. Global functions are not needed for class diagram
		theType, _ := getFieldType(decl.Recv.List[0].Type, p.AllImports, p.CurrentPackageName)
		theType = replacePackageConstant(theType, "")
		theType = strings.Trim(theType, "*.")
		structure := p.getOrCreateStruct(theType)
		if structure.Type == "" {
			structure.Type = "class"
		}

		fullName := fmt.Sprintf("%s%s", p.CurrentPackageName, theType)
		p.AllStructs[fullName] = struct{}{}
		structure.AddMethod(&ast.Field{
			Names:   []*ast.Ident{decl.Name},
			Doc:     decl.Doc,
			Type:    decl.Type,
			Tag:     nil,
			Comment: nil,
		}, p.AllImports)
	}
}

func handleGenDecStructType(p *ClassParser, typeName string, c *ast.StructType) {
	for _, f := range c.Fields.List {
		p.getOrCreateStruct(typeName).AddField(f, p.AllImports, p.CurrentPackageName)
	}
}

func handleGenDecInterfaceType(p *ClassParser, typeName string, c *ast.InterfaceType) {
	for _, f := range c.Methods.List {
		switch t := f.Type.(type) {
		case *ast.FuncType:
			p.getOrCreateStruct(typeName).AddMethod(f, p.AllImports)
			break
		case *ast.Ident:
			st := p.getOrCreateStruct(typeName)
			f, _ := getFieldType(t, p.AllImports, st.PackageName)
			f = replacePackageConstant(f, st.PackageName)
			st.AddToComposition(f)
			break
		}
	}
}

func (p *ClassParser) handleGenDecl(decl *ast.GenDecl) {
	if decl.Specs == nil || len(decl.Specs) < 1 {
		// This might be a type of General Declaration we do not know how to handle.
		return
	}
	for _, spec := range decl.Specs {
		p.processSpec(spec)
	}
}

func (p *ClassParser) processSpec(spec ast.Spec) {
	var typeName string
	var alias *Alias
	declarationType := "alias"
	switch v := spec.(type) {
	case *ast.TypeSpec:
		typeName = v.Name.Name
		switch c := v.Type.(type) {
		case *ast.StructType:
			declarationType = "class"
			handleGenDecStructType(p, typeName, c)
		case *ast.InterfaceType:
			declarationType = "interface"
			handleGenDecInterfaceType(p, typeName, c)
		default:
			basicType, _ := getFieldType(getBasicType(c), p.AllImports, p.CurrentPackageName)

			aliasType, _ := getFieldType(c, p.AllImports, p.CurrentPackageName)
			aliasType = replacePackageConstant(aliasType, "")
			if !IsPrimitiveString(typeName) {
				typeName = fmt.Sprintf("%s.%s", p.CurrentPackageName, typeName)
			}
			packageName := p.CurrentPackageName
			if IsPrimitiveString(basicType) {
				packageName = BuiltinPackageName
			}
			alias = getNewAlias(fmt.Sprintf("%s.%s", packageName, aliasType), p.CurrentPackageName, typeName)

		}
	default:
		// Not needed for class diagrams (Imports, global variables, regular functions, etc)
		return
	}
	p.getOrCreateStruct(typeName).Type = declarationType
	fullName := fmt.Sprintf("%s.%s", p.CurrentPackageName, typeName)
	switch declarationType {
	case "interface":
		p.AllInterfaces[fullName] = struct{}{}
	case "class":
		p.AllStructs[fullName] = struct{}{}
	case "alias":
		p.AllAliases[typeName] = alias
		if strings.Count(alias.Name, ".") > 1 {
			pack := strings.SplitN(alias.Name, ".", 2)
			if _, ok := p.AllRenamedStructs[pack[0]]; !ok {
				p.AllRenamedStructs[pack[0]] = map[string]string{}
			}
			renamedClass := GenerateRenamedStructName(pack[1])
			p.AllRenamedStructs[pack[0]][renamedClass] = pack[1]
		}
	}
	return
}

// If this element is an array or a pointer, this function will return the type that is closer to these
// two definitions. For example []***map[int] string will return map[int]string
func getBasicType(theType ast.Expr) ast.Expr {
	switch t := theType.(type) {
	case *ast.ArrayType:
		return getBasicType(t.Elt)
	case *ast.StarExpr:
		return getBasicType(t.X)
	case *ast.MapType:
		return getBasicType(t.Value)
	case *ast.ChanType:
		return getBasicType(t.Value)
	case *ast.Ellipsis:
		return getBasicType(t.Elt)
	}
	return theType
}

func (p *ClassParser) GetPackageName(t string, st *Struct) string {

	packageName := st.PackageName
	if IsPrimitiveString(t) {
		packageName = BuiltinPackageName
	}
	return packageName
}

// Returns an initialized struct of the given name or returns the existing one if it was already created
func (p *ClassParser) getOrCreateStruct(name string) *Struct {
	result, ok := p.Structure[p.CurrentPackageName][name]
	if !ok {
		result = &Struct{
			PackageName:         p.CurrentPackageName,
			Functions:           make([]*Function, 0),
			Fields:              make([]*Field, 0),
			Type:                "",
			Composition:         make(map[string]struct{}, 0),
			Extends:             make(map[string]struct{}, 0),
			Aggregations:        make(map[string]struct{}, 0),
			PrivateAggregations: make(map[string]struct{}, 0),
		}
		p.Structure[p.CurrentPackageName][name] = result
	}
	return result
}

// Returns an existing struct only if it was created. nil otherwhise
func (p *ClassParser) getStruct(structName string) *Struct {
	split := strings.Split(structName, ".")
	pack, ok := p.Structure[strings.Join(split[:len(split)-1], ".")]
	if !ok {
		return nil
	}
	return pack[split[len(split)-1]]
}

// SetRenderingOptions Sets the rendering options for the Render() Function
func (p *ClassParser) SetRenderingOptions(ro map[RenderingOption]interface{}) error {
	for option, val := range ro {
		switch option {
		case RenderAggregations:
			p.RenderingOptions.Aggregations = val.(bool)
		case RenderAliases:
			p.RenderingOptions.Aliases = val.(bool)
		case RenderCompositions:
			p.RenderingOptions.Compositions = val.(bool)
		case RenderFields:
			p.RenderingOptions.Fields = val.(bool)
		case RenderImplementations:
			p.RenderingOptions.Implementations = val.(bool)
		case RenderMethods:
			p.RenderingOptions.Methods = val.(bool)
		case RenderConnectionLabels:
			p.RenderingOptions.ConnectionLabels = val.(bool)
		case RenderTitle:
			p.RenderingOptions.Title = val.(string)
		case RenderNotes:
			p.RenderingOptions.Notes = val.(string)
		case AggregatePrivateMembers:
			p.RenderingOptions.AggregatePrivateMembers = val.(bool)
		case RenderPrivateMembers:
			p.RenderingOptions.PrivateMembers = val.(bool)
		default:
			return fmt.Errorf("Invalid Rendering option %v", option)
		}

	}
	return nil
}
func GenerateRenamedStructName(currentName string) string {
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	return reg.ReplaceAllString(currentName, "")
}
