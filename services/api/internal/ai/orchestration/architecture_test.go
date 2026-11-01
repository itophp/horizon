package orchestration

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBusinessCodeDoesNotUseADKWorkflowPrimitives(t *testing.T) {
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		aliases := map[string]bool{}
		for _, im := range file.Imports {
			value, _ := strconv.Unquote(im.Path.Value)
			if value == "github.com/cloudwego/eino/adk" {
				alias := "adk"
				if im.Name != nil {
					alias = im.Name.Name
				}
				if alias == "." {
					t.Errorf("%s: dot-import of ADK bypasses primitive checks", path)
				}
				aliases[alias] = true
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[name.Name] {
				return true
			}
			switch selector.Sel.Name {
			case "NewSequentialAgent", "NewParallelAgent", "NewLoopAgent":
				t.Errorf("%s: business sequencing must use Eino compose.Workflow/Graph", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
