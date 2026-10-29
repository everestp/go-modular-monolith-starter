package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestModuleBoundaries(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	modulesRoot := filepath.Join(root, "internal", "modules")
	fileset := token.NewFileSet()
	err := filepath.WalkDir(modulesRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(modulesRoot, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		module := parts[0]
		layer := "root"
		if len(parts) > 2 {
			layer = parts[1]
		}
		file, err := parser.ParseFile(fileset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			if strings.HasPrefix(importPath, "prod-go/internal/modules/") {
				importedModule := strings.TrimPrefix(importPath, "prod-go/internal/modules/")
				importedModule = strings.Split(importedModule, "/")[0]
				if importedModule != module {
					allowedPublicDependency := module == "auth" && importedModule == "users" && importPath == "prod-go/internal/modules/users"
					if !allowedPublicDependency {
						t.Errorf("%s imports another module (%s); cross-module imports must use an allowed public API", relative, importPath)
					}
				}
			}
			if layer == "domain" && strings.HasPrefix(importPath, "prod-go/") {
				t.Errorf("domain package %s imports project package %s", relative, importPath)
			}
			if layer == "service" && (strings.Contains(importPath, "/transport/") || strings.Contains(importPath, "/repository/")) {
				t.Errorf("service package %s imports an outer layer: %s", relative, importPath)
			}
			if layer == "transport" && strings.Contains(importPath, "/repository/") {
				t.Errorf("transport package %s imports a repository: %s", relative, importPath)
			}
			if layer == "repository" && (strings.Contains(importPath, "/transport/") || strings.Contains(importPath, "/service/")) {
				t.Errorf("repository package %s imports an inward layer: %s", relative, importPath)
			}
		}
		if layer != "repository" {
			ast.Inspect(file, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value := strings.Trim(literal.Value, "`\"")
				upper := strings.ToUpper(strings.TrimSpace(value))
				if strings.HasPrefix(upper, "SELECT ") && strings.Contains(upper, " FROM ") ||
					strings.HasPrefix(upper, "INSERT INTO ") ||
					strings.HasPrefix(upper, "UPDATE ") && strings.Contains(upper, " SET ") ||
					strings.HasPrefix(upper, "DELETE FROM ") ||
					strings.HasPrefix(upper, "CREATE TABLE ") ||
					strings.HasPrefix(upper, "ALTER TABLE ") ||
					strings.HasPrefix(upper, "DROP TABLE ") {
					t.Errorf("SQL literal found outside repository/postgres: %s", relative)
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module packages: %v", err)
	}
}
