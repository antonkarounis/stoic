package web

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"path"
	"reflect"
	"strings"
	"sync"
	"text/template/parse"
)

const defaultBaseTemplate = "base.html"

type TemplateManagerOptions struct {
	FS           fs.FS          // required: the filesystem to load templates from
	RootDir      string         // directory within FS containing page templates
	IncludeDir   string         // directory within FS containing shared includes
	FuncMap      map[string]any // custom template functions
	BaseTemplate string         // defaults to "base.html" if empty
	Reload       bool           // when true, reload templates on each request
}

// -----------------------------------

type TemplateManager struct {
	storedTemplates map[string]*template.Template
	baseExists      bool
	options         TemplateManagerOptions
	mu              sync.RWMutex // protects storedTemplates during reload
}

func NewTemplateManager(options TemplateManagerOptions) (*TemplateManager, error) {
	if options.FS == nil {
		return nil, errors.New("FS is required")
	}
	if options.FuncMap == nil {
		options.FuncMap = make(map[string]any)
	}
	if options.BaseTemplate == "" {
		options.BaseTemplate = defaultBaseTemplate
	}

	tm := &TemplateManager{
		options: options,
	}

	if err := tm.loadTemplates(); err != nil {
		return nil, err
	}
	return tm, nil
}

func (tm *TemplateManager) loadTemplates() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.storedTemplates = make(map[string]*template.Template)
	tm.baseExists = false

	// load includes from IncludeDir
	includes := template.New("root").Funcs(tm.options.FuncMap)
	if tm.options.IncludeDir != "" {
		includeEntries, err := fs.ReadDir(tm.options.FS, tm.options.IncludeDir)
		if err != nil {
			return fmt.Errorf("reading include dir: %w", err)
		}
		for _, entry := range includeEntries {
			if entry.IsDir() {
				continue
			}
			includePath := path.Join(tm.options.IncludeDir, entry.Name())
			content, err := fs.ReadFile(tm.options.FS, includePath)
			if err != nil {
				return fmt.Errorf("reading include %s: %w", includePath, err)
			}
			_, err = includes.New(entry.Name()).Parse(string(content))
			if err != nil {
				return fmt.Errorf("parsing include %s: %w", includePath, err)
			}
		}
	}

	if base := includes.Lookup(tm.options.BaseTemplate); base != nil {
		tm.baseExists = true
	}

	// load page templates from RootDir
	err := fs.WalkDir(tm.options.FS, tm.options.RootDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Get path relative to RootDir
		relativePath, err := relPath(tm.options.RootDir, filePath)
		if err != nil {
			return err
		}

		content, err := fs.ReadFile(tm.options.FS, filePath)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", filePath, err)
		}

		newTemplate, err := includes.Clone()
		if err != nil {
			return err
		}

		_, err = newTemplate.New(relativePath).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", filePath, err)
		}

		tm.storedTemplates[relativePath] = newTemplate
		return nil
	})

	return err
}

// relPath returns the relative path from base to target using path (not filepath)
func relPath(base, target string) (string, error) {
	if base == "." || base == "" {
		return target, nil
	}
	if !strings.HasPrefix(target, base) {
		return "", fmt.Errorf("target %q is not under base %q", target, base)
	}
	rel := strings.TrimPrefix(target, base)
	rel = strings.TrimPrefix(rel, "/")
	return rel, nil
}

// getTemplateForExecution returns the template, reloading all templates first if Reload is enabled
func (tm *TemplateManager) getTemplateForExecution(templatePath string) (*template.Template, error) {
	if tm.options.Reload {
		if err := tm.loadTemplates(); err != nil {
			return nil, fmt.Errorf("reloading templates: %w", err)
		}
	}

	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tmpl := tm.storedTemplates[templatePath]
	if tmpl == nil {
		return nil, errors.New("couldn't find template: " + templatePath)
	}
	return tmpl, nil
}

// usesBaseLayout returns true if the template defines a "content" block,
// indicating it should be rendered via the base layout
func usesBaseLayout(tmpl *template.Template) bool {
	return tmpl.Lookup("content") != nil
}

func (tm *TemplateManager) getExecutor(templatePath string, exampleModel any) (*TemplateExecutor, error) {
	tm.mu.RLock()
	tmpl := tm.storedTemplates[templatePath]
	tm.mu.RUnlock()

	if tmpl == nil {
		return nil, errors.New("couldn't find template: " + templatePath)
	}

	if err := validateViewModelAllBlocks(exampleModel, tmpl, templatePath); err != nil {
		return nil, fmt.Errorf("couldn't validate view model for [%v]: %v", templatePath, err.Error())
	}

	return &TemplateExecutor{
		manager:          tm,
		templateName:     templatePath,
		baseTemplateName: tm.options.BaseTemplate,
	}, nil
}

func (tm *TemplateManager) GetTemplate(templatePath string, exampleModel any) (*template.Template, error) {
	if tm.options.Reload {
		if err := tm.loadTemplates(); err != nil {
			return nil, fmt.Errorf("reloading templates: %w", err)
		}
	}

	tm.mu.RLock()
	tmpl := tm.storedTemplates[templatePath]
	tm.mu.RUnlock()

	if tmpl == nil {
		return nil, errors.New("couldn't find template: " + templatePath)
	}

	if err := validateViewModelAllBlocks(exampleModel, tmpl, templatePath); err != nil {
		return nil, fmt.Errorf("couldn't validate view model for [%v]: %v", templatePath, err.Error())
	}

	return tmpl, nil
}

func (tm *TemplateManager) TemplateRoute(
	templatePath string,
	exampleModel any,
	fn func(w http.ResponseWriter, r *http.Request, tmpl *template.Template),
) func(w http.ResponseWriter, r *http.Request) {
	tmpl, err := tm.GetTemplate(templatePath, exampleModel)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, tmpl)
	}
}

func (tm *TemplateManager) ExecutorRoute(
	templatePath string,
	exampleModel any,
	fn func(w http.ResponseWriter, r *http.Request, te *TemplateExecutor),
) func(w http.ResponseWriter, r *http.Request) {

	executor, err := tm.getExecutor(templatePath, exampleModel)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, executor)
	}
}

// -----------------------------------

type TemplateExecutor struct {
	manager          *TemplateManager
	templateName     string
	baseTemplateName string
}

func (te *TemplateExecutor) ExecuteToString(data any) (string, error) {
	var buffer bytes.Buffer
	if err := te.ExecuteToWriter(&buffer, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (te *TemplateExecutor) ExecuteToWriter(writer io.Writer, data any) error {
	tmpl, err := te.manager.getTemplateForExecution(te.templateName)
	if err != nil {
		return err
	}

	// If template defines "content" block, execute via base layout
	// Otherwise execute the page template directly
	execName := te.templateName
	if usesBaseLayout(tmpl) && te.manager.baseExists {
		execName = te.baseTemplateName
	}

	if err := tmpl.ExecuteTemplate(writer, execName, data); err != nil {
		return fmt.Errorf("error executing template [%v]: %v", te.templateName, err.Error())
	}

	return nil
}

// validateViewModelAllBlocks validates the data model against all blocks defined by the page template.
// This catches fields used in any block (content, nav, head, title, etc.), not just "content".
func validateViewModelAllBlocks(data interface{}, tmpl *template.Template, templatePath string) error {
	// Collect all block names defined by this page template
	blockNames := []string{templatePath}
	knownBlocks := []string{"content", "nav", "head", "title"}
	for _, name := range knownBlocks {
		if t := tmpl.Lookup(name); t != nil {
			blockNames = append(blockNames, name)
		}
	}

	// Extract fields used across all blocks
	rootTemplateField := newTemplateField("Root")
	for _, name := range blockNames {
		block := tmpl.Lookup(name)
		if block != nil && block.Tree != nil {
			extractFieldsFromTemplate(tmpl, block.Tree.Root, rootTemplateField)
		}
	}

	rootStructField := extractFieldsFromData(data)
	missing, extra := compareTemplateFields(rootTemplateField, rootStructField)

	if len(extra) == 0 && len(missing) == 0 {
		return nil
	}

	var sb strings.Builder
	if len(extra) > 0 {
		sb.WriteString("extra fields [")
		sb.WriteString(strings.Join(extra, ", "))
		sb.WriteString("] ")
	}
	if len(missing) > 0 {
		sb.WriteString("missing fields [")
		sb.WriteString(strings.Join(missing, ", "))
		sb.WriteString("]")
	}
	return errors.New(sb.String())
}

func validateViewModel(data interface{}, tmpl *template.Template, templateName string) error {
	missing, extra := compareViewModel(data, tmpl, templateName)
	if len(extra) == 0 && len(missing) == 0 {
		return nil
	}

	var sb strings.Builder
	if len(extra) > 0 {
		sb.WriteString("extra fields [")
		sb.WriteString(strings.Join(extra, ", "))
		sb.WriteString("] ")
	}
	if len(missing) > 0 {
		sb.WriteString("missing fields [")
		sb.WriteString(strings.Join(missing, ", "))
		sb.WriteString("]")
	}
	return errors.New(sb.String())
}

type templateField struct {
	Name     string                    // Name of the field
	Children map[string]*templateField // Nested fields (e.g., for structs or maps)
}

func newTemplateField(name string) *templateField {
	return &templateField{
		Name:     name,
		Children: make(map[string]*templateField),
	}
}

func (tf *templateField) addChild(name string) *templateField {
	child, exists := tf.Children[name]
	if !exists {
		child = newTemplateField(name)
		tf.Children[name] = child
	}
	return child
}

func extractFieldsFromTemplate(root *template.Template, n parse.Node, parentField *templateField) {
	switch node := n.(type) {
	case *parse.PipeNode:
		for _, cmd := range node.Cmds {
			for _, arg := range cmd.Args {
				if field, ok := arg.(*parse.FieldNode); ok {
					// Add the field to the parent field's children
					fieldParts := field.Ident
					current := parentField
					for _, part := range fieldParts {
						current = current.addChild(part)
					}
				}
			}
		}
	case *parse.ActionNode:
		for _, cmd := range node.Pipe.Cmds {
			for _, arg := range cmd.Args {
				if field, ok := arg.(*parse.FieldNode); ok {
					// Add the field to the parent field's children
					fieldParts := field.Ident
					current := parentField
					for _, part := range fieldParts {
						current = current.addChild(part)
					}
				}
			}
		}
	case *parse.TemplateNode:
		// Handle included templates ({{ template "name" .Field }})
		if node.Pipe != nil {
			for _, cmd := range node.Pipe.Cmds {
				for _, arg := range cmd.Args {
					if field, ok := arg.(*parse.FieldNode); ok {
						// Locate the parent for the included template's fields
						fieldParts := field.Ident
						current := parentField
						for _, part := range fieldParts {
							current = current.addChild(part)
						}
						// Recursively extract fields from the included template under this parent
						if tmpl := root.Lookup(node.Name); tmpl != nil && tmpl.Tree != nil {
							extractFieldsFromTemplate(root, tmpl.Tree.Root, current)
						}
					} else if _, ok := arg.(*parse.DotNode); ok {
						// Recursively extract fields from the included template under this parent
						if tmpl := root.Lookup(node.Name); tmpl != nil && tmpl.Tree != nil {
							extractFieldsFromTemplate(root, tmpl.Tree.Root, parentField)
						}
					}
				}
			}
		} else {
			// If no specific argument is passed, inherit from the parent
			if tmpl := root.Lookup(node.Name); tmpl != nil && tmpl.Tree != nil {
				extractFieldsFromTemplate(root, tmpl.Tree.Root, parentField)
			}
		}
	case *parse.ListNode:
		for _, child := range node.Nodes {
			extractFieldsFromTemplate(root, child, parentField)
		}
	case *parse.IfNode:
		extractFieldsFromTemplate(root, node.List, parentField)
		if node.ElseList != nil {
			extractFieldsFromTemplate(root, node.ElseList, parentField)
		}
	case *parse.RangeNode:
		extractFieldsFromTemplate(root, node.Pipe, parentField)
	case *parse.WithNode:
		extractFieldsFromTemplate(root, node.List, parentField)
	}
}

func extractFieldsFromData(v any) *templateField {
	root := newTemplateField("Root")
	if v == nil {
		return root // empty field set - will report missing fields if template expects data
	}

	val := reflect.ValueOf(v)
	typ := val.Type()

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
		val = val.Elem()
	}

	switch typ.Kind() {
	case reflect.Map:
		for _, key := range val.MapKeys() {
			name := fmt.Sprintf("%v", key.Interface())
			child := root.addChild(name)
			elem := val.MapIndex(key)
			if elem.IsValid() && elem.CanInterface() {
				extractFieldHelper(reflect.TypeOf(elem.Interface()), child)
			}
		}
	case reflect.Struct:
		extractFieldHelper(typ, root)
	}

	return root
}

func extractFieldHelper(typ reflect.Type, parentField *templateField) {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // Skip unexported fields
			continue
		}

		child := parentField.addChild(field.Name)
		if field.Type.Kind() == reflect.Struct {
			extractFieldHelper(field.Type, child)
		}
	}
}

func compareViewModel(data interface{}, tmpl *template.Template, templateName string) (missing []string, extra []string) {
	// Extract fields from the template
	rootTemplateField := newTemplateField("Root")
	tmplTree := tmpl.Lookup(templateName)
	if tmplTree == nil || tmplTree.Tree == nil {
		panic(fmt.Sprintf("template %q not found", templateName))
	}
	extractFieldsFromTemplate(tmpl, tmplTree.Tree.Root, rootTemplateField)

	// Extract fields from the struct
	rootStructField := extractFieldsFromData(data)

	// Compare the two field sets
	return compareTemplateFields(rootTemplateField, rootStructField)
}

func compareTemplateFields(templateField, structField *templateField) (missing []string, extra []string) {
	for name, child := range templateField.Children {
		if _, ok := structField.Children[name]; !ok {
			missing = append(missing, structField.Name+"->"+name)
		} else {
			m, e := compareTemplateFields(child, structField.Children[name])
			missing = append(missing, m...)
			extra = append(extra, e...)
		}
	}
	for name := range structField.Children {
		if _, ok := templateField.Children[name]; !ok {
			extra = append(extra, templateField.Name+"->"+name)
		}
	}
	return missing, extra
}
