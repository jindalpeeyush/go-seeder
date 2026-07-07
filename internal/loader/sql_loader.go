package loader

import (
	"fmt"
	"os"
	"strings"
)

// loadSQL parses a SQL seed file. The file should contain:
//   - First line: driver header (e.g., "-- driver: postgres")
//   - Remaining: raw SQL statements separated by semicolons
func loadSQL(seed *SeedFile) error {
	data, err := os.ReadFile(seed.Path)
	if err != nil {
		return fmt.Errorf("failed to read SQL file %s: %w", seed.Path, err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return nil
	}

	// Extract driver from first line
	seed.Driver = extractSQLDriver(content)

	// Parse SQL statements
	seed.Statements = splitSQLStatements(content)
	return nil
}

// extractSQLDriver reads the driver from a "-- driver: <name>" comment.
func extractSQLDriver(content string) string {
	lines := strings.SplitN(content, "\n", 5)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-- driver:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "-- driver:"))
		}
	}
	return ""
}

// splitSQLStatements splits SQL content into individual statements,
// skipping driver header comments.
func splitSQLStatements(content string) []string {
	// Clean out driver comments first
	var cleanedLines []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "-- driver:") {
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}
	cleanedContent := strings.Join(cleanedLines, "\n")

	if strings.TrimSpace(cleanedContent) == "" {
		return nil
	}

	var statements []string
	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false

	runes := []rune(cleanedContent)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		next := rune(0)
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		if !inSingleQuote && !inDoubleQuote && !inLineComment && ch == '/' && next == '*' {
			inBlockComment = true
			current.WriteRune(ch)
			continue
		}
		if inBlockComment && ch == '*' && next == '/' {
			inBlockComment = false
			current.WriteRune(ch)
			current.WriteRune(next)
			i++
			continue
		}
		if inBlockComment {
			current.WriteRune(ch)
			continue
		}
		if !inSingleQuote && !inDoubleQuote && ch == '-' && next == '-' {
			inLineComment = true
			current.WriteRune(ch)
			continue
		}
		if inLineComment && ch == '\n' {
			inLineComment = false
			current.WriteRune(ch)
			continue
		}
		if inLineComment {
			current.WriteRune(ch)
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
		}
		if ch == ';' && !inSingleQuote && !inDoubleQuote {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}

	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}
