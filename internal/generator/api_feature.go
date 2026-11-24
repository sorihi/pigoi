package generator

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/sorihi/pigoi/templates"
)

func GenerateFeature(name string) error {
	return generateFeature(name, "create-api-feature", templates.FeatureFS, "go-api-feature", false)
}

func GenerateWorkerFeature(name string) error {
	return generateFeature(name, "create-worker-feature", templates.WorkerFS, "go-worker-feature/feature", true)
}

func generateFeature(name, command string, sourceFS fs.FS, templateRoot string, worker bool) error {
	if !token.IsIdentifier(name) || name == "_" || name == "main" {
		return fmt.Errorf("feature name %q must be an importable Go package name", name)
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run %s from the generated project's root: %w", command, err)
	}
	module, err := readModulePath()
	if err != nil {
		return err
	}

	provider := filepath.Join("internal", "provider.go")
	target := filepath.Join("internal", name)
	lock, err := os.OpenFile(provider+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("lock provider for feature generation: %w", err)
	}
	defer os.Remove(lock.Name())
	lock.Close()

	source, err := os.ReadFile(provider)
	if err != nil {
		return err
	}
	info, err := os.Stat(provider)
	if err != nil {
		return err
	}
	updated, err := wireFeature(source, name, module, worker)
	if err != nil {
		return fmt.Errorf("update %s: %w", provider, err)
	}

	if err := os.Mkdir(target, 0755); err != nil {
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_ = os.RemoveAll(target)
		}
	}()

	if err := render(sourceFS, templateRoot, target, Options{Name: name, Module: module}); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(provider), ".provider-*.go")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := temp.Write(updated); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), provider); err != nil {
		return err
	}
	completed = true
	return nil
}

func readModulePath() (string, error) {
	output, err := exec.Command("go", "mod", "edit", "-json", "go.mod").Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("read go.mod: %w\n%s", err, exit.Stderr)
		}
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	var mod struct {
		Module struct {
			Path string
		}
	}
	if err := json.Unmarshal(output, &mod); err != nil {
		return "", fmt.Errorf("decode go.mod metadata: %w", err)
	}
	if mod.Module.Path == "" {
		return "", fmt.Errorf("go.mod has no module directive")
	}
	return mod.Module.Path, nil
}

func wireFeature(source []byte, name, module string, worker bool) ([]byte, error) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "provider.go", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	importPath := strconv.Quote(module + "/internal/" + name)
	alias := "feature_" + name
	hasRabbitMQ := false
	for _, spec := range file.Imports {
		if spec.Path.Value == strconv.Quote(module+"/pkg/rabbitmq") {
			hasRabbitMQ = true
		}
		if spec.Path.Value == importPath {
			return nil, fmt.Errorf("feature %q is already imported", name)
		}
	}

	var imports *ast.GenDecl
	var providers, invokes *ast.CallExpr
	var routes *ast.FuncDecl
	for _, decl := range file.Decls {
		if group, ok := decl.(*ast.GenDecl); ok && group.Tok == token.IMPORT && group.Lparen.IsValid() {
			imports = group
		}
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil {
			continue
		}
		switch fn.Name.Name {
		case "Run":
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if ok {
					switch {
					case isSelector(call.Fun, "fx", "Provide"):
						providers = call
						return false
					case isSelector(call.Fun, "fx", "Invoke"):
						invokes = call
						return false
					}
				}
				return true
			})
		case "registerRoutes":
			routes = fn
		}
	}
	if imports == nil || providers == nil {
		return nil, fmt.Errorf("expected an import block and fx.Provide in Run")
	}

	providerArgs := fmt.Sprintf(
		"\nfx.Annotate(%[1]s.NewService, fx.As(new(%[1]s.IService))),\nfx.Annotate(%[1]s.NewRepository, fx.As(new(%[1]s.IRepository))),\n", alias)
	edits := []sourceEdit{{imports.Rparen, fmt.Sprintf("\n%s %s\n", alias, importPath)}}

	if worker {
		if !hasRabbitMQ || invokes == nil {
			return nil, fmt.Errorf("expected the RabbitMQ provider and fx.Invoke in Run from the current API project template")
		}
		providerArgs += fmt.Sprintf("%s.NewConsumer,\n", alias)
		edits = append(edits, sourceEdit{invokes.Lparen + 1, alias + ".RegisterBroker, "})
	} else {
		if routes == nil {
			return nil, fmt.Errorf("expected a registerRoutes function")
		}
		params := routes.Type.Params
		if len(params.List) == 0 || len(params.List[0].Names) != 1 {
			return nil, fmt.Errorf("expected a named router parameter in registerRoutes")
		}
		router := params.List[0].Names[0].Name
		route := alias + "Route"
		providerArgs += fmt.Sprintf("%[1]s.NewDelivery,\n%[1]s.NewRoute,\n", alias)
		edits = append(edits,
			sourceEdit{params.List[0].End(), fmt.Sprintf(", %s *%s.Route", route, alias)},
			sourceEdit{routes.Body.Rbrace, fmt.Sprintf("\n%s.RegisterRoutes(%s.Group(%q))\n", route, router, "/"+name)},
		)
	}
	edits = append(edits, sourceEdit{providers.Lparen + 1, providerArgs})

	sort.Slice(edits, func(i, j int) bool { return edits[i].pos > edits[j].pos })
	updated := string(source)
	for _, edit := range edits {
		offset := files.Position(edit.pos).Offset
		updated = updated[:offset] + edit.text + updated[offset:]
	}
	return format.Source([]byte(updated))
}

type sourceEdit struct {
	pos  token.Pos
	text string
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg
}
