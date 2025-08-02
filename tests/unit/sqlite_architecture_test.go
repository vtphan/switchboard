package unit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteArchitecturalCompliance(t *testing.T) {
	t.Run("SQLite implementation file exists", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		_, err := os.Stat(filePath)
		assert.NoError(t, err, "SQLite implementation file must exist at %s", filePath)
	})

	t.Run("Batching is built into SQLite implementation", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		require.NoError(t, err, "SQLite implementation file must exist")
		
		contentStr := string(content)
		assert.Contains(t, contentStr, "flushMessageBatch", "SQLite implementation must have built-in batching")
		assert.Contains(t, contentStr, "config.DatabaseBatchSize", "SQLite implementation must use DatabaseBatchSize")
		assert.Contains(t, contentStr, "config.DatabaseFlushInterval", "SQLite implementation must use DatabaseFlushInterval")
	})
}

func TestSQLitePackageStructure(t *testing.T) {
	t.Run("SQLite file has correct package declaration", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		assert.Contains(t, string(content), "package database", "SQLite file must have correct package declaration")
	})

}

func TestSQLiteImportRestrictions(t *testing.T) {
	forbiddenImports := []string{
		"internal/websocket",
		"internal/session", 
		"internal/message",
		"web/",
	}

	t.Run("SQLite file has no forbidden imports", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		contentStr := string(content)
		for _, forbidden := range forbiddenImports {
			assert.NotContains(t, contentStr, forbidden, 
				"SQLite implementation must not import %s (architectural layer violation)", forbidden)
		}
	})

}

func TestSQLiteDatabaseManagerStructure(t *testing.T) {
	t.Run("SQLiteDatabaseManager struct exists with exact fields", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
		require.NoError(t, err, "SQLite file must be valid Go code")

		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok && typeSpec.Name.Name == "SQLiteDatabaseManager" {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					found = true
					
					// Check for exact required fields
					requiredFields := []string{
						"db", "writeChannel", "deadLetterQueue", "metrics", "stopCh", "wg",
					}
					
					foundFields := make(map[string]bool)
					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							foundFields[name.Name] = true
						}
					}
					
					for _, required := range requiredFields {
						assert.True(t, foundFields[required], 
							"SQLiteDatabaseManager must have field %s", required)
					}
				}
			}
			return true
		})
		
		assert.True(t, found, "SQLiteDatabaseManager struct must exist")
	})
}

func TestWriteRequestStructure(t *testing.T) {
	t.Run("writeRequest struct exists with exact fields", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
		require.NoError(t, err, "SQLite file must be valid Go code")

		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok && typeSpec.Name.Name == "writeRequest" {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					found = true
					
					// Check for exact required fields
					requiredFields := []string{"operation", "data", "responseCh"}
					
					foundFields := make(map[string]bool)
					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							foundFields[name.Name] = true
						}
					}
					
					for _, required := range requiredFields {
						assert.True(t, foundFields[required], 
							"writeRequest must have field %s", required)
					}
				}
			}
			return true
		})
		
		assert.True(t, found, "writeRequest struct must exist")
	})
}

func TestDatabaseMetricsStructure(t *testing.T) {
	t.Run("DatabaseMetrics struct exists with exact fields", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
		require.NoError(t, err, "SQLite file must be valid Go code")

		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if typeSpec, ok := n.(*ast.TypeSpec); ok && typeSpec.Name.Name == "DatabaseMetrics" {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					found = true
					
					// Check for exact required fields (including new batch metrics)
					requiredFields := []string{
						"SuccessfulWrites", "FailedWrites", "RetriedWrites", 
						"DeadLetterCount", "PermanentLossCount",
						"BatchesWritten", "MessagesPerBatch", "BatchFlushBySize", "BatchFlushByTime",
					}
					
					foundFields := make(map[string]bool)
					for _, field := range structType.Fields.List {
						for _, name := range field.Names {
							foundFields[name.Name] = true
						}
					}
					
					for _, required := range requiredFields {
						assert.True(t, foundFields[required], 
							"DatabaseMetrics must have field %s", required)
					}
				}
			}
			return true
		})
		
		assert.True(t, found, "DatabaseMetrics struct must exist")
	})
}

func TestDatabaseManagerInterfaceImplementation(t *testing.T) {
	t.Run("SQLiteDatabaseManager implements DatabaseManager interface", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		contentStr := string(content)
		
		// Check that all interface methods are implemented
		requiredMethods := []string{
			"CreateSession", "UpdateSession", "GetActiveSession",
			"WriteMessage", "WriteBatch", "GetSessionMessages",
			"Start", "Stop",
		}
		
		for _, method := range requiredMethods {
			// Look for method signature with receiver
			assert.Contains(t, contentStr, method, 
				"SQLiteDatabaseManager must implement %s method", method)
		}
	})
}

func TestSQLiteConfigurationUsage(t *testing.T) {
	t.Run("SQLite implementation uses configuration constants", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		contentStr := string(content)
		
		// Check for usage of configuration constants
		requiredConfigs := []string{
			"config.DatabaseWriteBuffer",
			"config.DeadLetterQueueSize", 
			"config.SQLiteConfiguration",
			"config.DatabaseMaxRetries",
		}
		
		for _, configConst := range requiredConfigs {
			assert.Contains(t, contentStr, configConst, 
				"SQLite implementation must use %s", configConst)
		}
	})
}

func TestSingleWriterPatternArchitecture(t *testing.T) {
	t.Run("Write methods use channel-based single writer pattern", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Skip("SQLite file not implemented yet")
		}

		contentStr := string(content)
		
		// Check for evidence of single-writer pattern
		assert.Contains(t, contentStr, "writeChannel", 
			"Implementation must use writeChannel for single-writer pattern")
		assert.Contains(t, contentStr, "writeLoop", 
			"Implementation must have writeLoop goroutine")
		
		// Ensure write methods don't directly access database
		writeMethods := []string{
			"CreateSession", "UpdateSession", "WriteMessage", "WriteBatch",
		}
		
		for _, method := range writeMethods {
			methodStart := strings.Index(contentStr, "func (.*) "+method)
			if methodStart != -1 {
				// Extract method body (rough check)
				methodEnd := strings.Index(contentStr[methodStart:], "\nfunc ")
				if methodEnd == -1 {
					methodEnd = len(contentStr)
				} else {
					methodEnd += methodStart
				}
				
				methodBody := contentStr[methodStart:methodEnd]
				assert.NotContains(t, methodBody, "db.Exec", 
					"Write method %s must not directly execute SQL (violates single-writer pattern)", method)
				assert.NotContains(t, methodBody, "db.Query", 
					"Write method %s must not directly query SQL (violates single-writer pattern)", method)
			}
		}
	})
}

func TestBatchingArchitecture(t *testing.T) {
	t.Run("Built-in message batching implementation exists", func(t *testing.T) {
		filePath := "/Users/vinhthuyphan/Apps/switchboard/internal/database/sqlite.go"
		content, err := os.ReadFile(filePath)
		require.NoError(t, err, "SQLite file must exist")

		contentStr := string(content)
		
		// Check for built-in batching components
		assert.Contains(t, contentStr, "flushMessageBatch", 
			"SQLite implementation must have flushMessageBatch method")
		assert.Contains(t, contentStr, "messageBatch", 
			"SQLite implementation must have message batching in writeLoop")
		assert.Contains(t, contentStr, "config.DatabaseBatchSize", 
			"SQLite implementation must use DatabaseBatchSize constant")
		assert.Contains(t, contentStr, "config.DatabaseFlushInterval", 
			"SQLite implementation must use DatabaseFlushInterval constant")
		assert.Contains(t, contentStr, "BatchFlushBySize", 
			"SQLite implementation must track batch flush metrics")
		assert.Contains(t, contentStr, "BatchFlushByTime", 
			"SQLite implementation must track timer-based flush metrics")
	})
}