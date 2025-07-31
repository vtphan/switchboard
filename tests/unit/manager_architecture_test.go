package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDatabaseManagerFileExists verifies the manager.go file exists and is parseable
func TestDatabaseManagerFileExists(t *testing.T) {
	// This test will fail initially since manager.go doesn't exist
	managerPath := filepath.Join("..", "..", "internal", "database", "manager.go")
	fset := token.NewFileSet()
	
	_, err := parser.ParseFile(fset, managerPath, nil, parser.ParseComments)
	require.NoError(t, err, "manager.go should exist and be parseable")
}

// TestDatabaseManagerNoForbiddenImports ensures manager.go doesn't import forbidden packages
func TestDatabaseManagerNoForbiddenImports(t *testing.T) {
	managerPath := filepath.Join("..", "..", "internal", "database", "manager.go")
	fset := token.NewFileSet()
	
	file, err := parser.ParseFile(fset, managerPath, nil, parser.ParseComments)
	require.NoError(t, err, "manager.go should be parseable")
	
	forbiddenImports := []string{
		"internal/session",
		"internal/message", 
		"internal/websocket",
	}
	
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		for _, forbidden := range forbiddenImports {
			assert.NotContains(t, importPath, forbidden, 
				"manager.go should not import %s to maintain architectural boundaries", forbidden)
		}
	}
}

// TestDatabaseManagerInterfaceExists validates that DatabaseManager interface exists
func TestDatabaseManagerInterfaceExists(t *testing.T) {
	// Import the database package and verify DatabaseManager interface exists
	managerPath := filepath.Join("..", "..", "internal", "database", "manager.go")
	fset := token.NewFileSet()
	
	file, err := parser.ParseFile(fset, managerPath, nil, parser.ParseComments)
	require.NoError(t, err, "manager.go should be parseable")
	
	// Look for DatabaseManager interface declaration
	found := false
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if typeSpec.Name.Name == "DatabaseManager" {
						_, isInterface := typeSpec.Type.(*ast.InterfaceType)
						assert.True(t, isInterface, "DatabaseManager should be an interface")
						found = true
						break
					}
				}
			}
		}
	}
	
	assert.True(t, found, "DatabaseManager interface should be declared in manager.go")
}

// TestDatabaseManagerMethodSignatures validates all interface method signatures
func TestDatabaseManagerMethodSignatures(t *testing.T) {
	// Parse the manager.go file and extract interface methods
	managerPath := filepath.Join("..", "..", "internal", "database", "manager.go")
	fset := token.NewFileSet()
	
	file, err := parser.ParseFile(fset, managerPath, nil, parser.ParseComments)
	require.NoError(t, err, "manager.go should be parseable")
	
	// Find DatabaseManager interface and verify it has exactly 8 methods
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.Name.Name == "DatabaseManager" {
					if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						// Should have exactly 9 methods (including WaitForPendingWrites)
						assert.Equal(t, 9, len(interfaceType.Methods.List), "DatabaseManager should have exactly 9 methods")
						
						// Verify specific method names exist
						methodNames := make(map[string]bool)
						for _, method := range interfaceType.Methods.List {
							if len(method.Names) > 0 {
								methodNames[method.Names[0].Name] = true
							}
						}
						
						expectedMethods := []string{
							"CreateSession", "UpdateSession", "GetActiveSession",
							"WriteMessage", "WriteBatch", "GetSessionMessages",
							"Start", "Stop",
						}
						
						for _, methodName := range expectedMethods {
							assert.True(t, methodNames[methodName], "Method %s should exist in DatabaseManager interface", methodName)
						}
						
						return
					}
				}
			}
		}
	}
	
	assert.Fail(t, "DatabaseManager interface not found")
}

// TestDatabaseManagerPackageDeclaration verifies correct package declaration
func TestDatabaseManagerPackageDeclaration(t *testing.T) {
	managerPath := filepath.Join("..", "..", "internal", "database", "manager.go")
	fset := token.NewFileSet()
	
	file, err := parser.ParseFile(fset, managerPath, nil, parser.ParseComments)
	require.NoError(t, err, "manager.go should be parseable")
	
	assert.Equal(t, "database", file.Name.Name, "Package declaration should be 'database'")
}

// TestDatabaseManagerNoImplementation ensures interface file contains no concrete implementations
func TestDatabaseManagerNoImplementation(t *testing.T) {
	// This test ensures the file only contains interface definitions, no struct types
	managerPath := filepath.Join("..", "..", "internal", "database", "manager.go")
	fset := token.NewFileSet()
	
	file, err := parser.ParseFile(fset, managerPath, nil, parser.ParseComments)
	require.NoError(t, err, "manager.go should be parseable")
	
	// Count type declarations and ensure they're all interfaces
	typeCount := 0
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					typeCount++
					// Verify it's an interface, not a struct
					_, isInterface := typeSpec.Type.(*ast.InterfaceType)
					assert.True(t, isInterface, "Type %s should be an interface, not a struct", typeSpec.Name.Name)
				}
			}
		}
	}
	
	// Should have exactly one type declaration (the DatabaseManager interface)
	assert.Equal(t, 1, typeCount, "manager.go should contain exactly one type declaration (DatabaseManager interface)")
}