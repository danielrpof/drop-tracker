// Package sqlscan holds the three stdlib-only parsing engines the
// migration-check tool drives: a SQL comment/quote lexer, a structural DDL
// parser (Parse), and the D-15 query-reference extractor (QueryColumnRefs).
// It reads no files and spawns no subprocess -- strings in, typed values
// out. Every policy, I/O, and git concern stays in cmd/migration-check.
package sqlscan

import "strings"

// RawStatement is the lexer's unit of output and the Parse fallback for any
// statement Parse does not structurally recognise.
type RawStatement struct {
	Text string // trimmed statement text, semicolon removed
	Line int    // 1-based line of the statement's first non-blank byte
}

// StripComments removes `--` line comments and /* */ block comments,
// replacing their bytes with spaces while preserving every newline so
// 1-based line numbers computed downstream stay accurate. String literals
// and $tag$...$tag$ dollar-quoted spans pass through untouched.
func StripComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		switch {
		case c == '-' && i+1 < n && src[i+1] == '-':
			for i < n && src[i] != '\n' {
				b.WriteByte(' ')
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			b.WriteByte(' ')
			b.WriteByte(' ')
			i += 2
			for i < n && (src[i] != '*' || i+1 >= n || src[i+1] != '/') {
				if src[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
			if i+1 < n {
				b.WriteByte(' ')
				b.WriteByte(' ')
				i += 2
			}
		case c == '\'':
			b.WriteByte(c)
			i++
			i = copySingleQuoted(&b, src, i)
		case c == '$':
			if tag, ok := dollarTagAt(src, i); ok {
				end := copyDollarQuoted(&b, src, i, tag)
				i = end
			} else {
				b.WriteByte(c)
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// copySingleQuoted copies a '...' string literal body (with '' escapes)
// starting just after the opening quote, returning the index past the
// closing quote.
func copySingleQuoted(b *strings.Builder, src string, i int) int {
	n := len(src)
	for i < n {
		c := src[i]
		b.WriteByte(c)
		if c == '\'' {
			if i+1 < n && src[i+1] == '\'' {
				b.WriteByte(src[i+1])
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
}

// dollarTagAt reports whether src[i:] begins a $tag$ delimiter and returns
// the full delimiter (e.g. "$$" or "$body$").
func dollarTagAt(src string, i int) (string, bool) {
	rest := src[i+1:]
	end := strings.IndexByte(rest, '$')
	if end < 0 || !isValidDollarTag(rest[:end]) {
		return "", false
	}
	return src[i : i+1+end+1], true
}

func isValidDollarTag(tag string) bool {
	for _, r := range tag {
		if r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// copyDollarQuoted copies from the opening tag through the matching closing
// tag (inclusive), returning the index just past it. If no closing tag
// exists the rest of the source is copied verbatim.
func copyDollarQuoted(b *strings.Builder, src string, i int, tag string) int {
	b.WriteString(tag)
	i += len(tag)
	closeIdx := strings.Index(src[i:], tag)
	if closeIdx < 0 {
		b.WriteString(src[i:])
		return len(src)
	}
	b.WriteString(src[i : i+closeIdx+len(tag)])
	return i + closeIdx + len(tag)
}

// SplitStatements splits comment-stripped SQL on `;`, respecting '...'
// string literals and $tag$...$tag$ dollar-quoting so an embedded semicolon
// never ends a statement early.
//
// This hand-rolled single-quote/dollar-quote scanner is a near-duplicate of
// StripComments' own -- kept separate to make the extraction
// behavior-preserving; unifying them is a filed follow-up.
func SplitStatements(src string) []RawStatement {
	var stmts []RawStatement
	var cur strings.Builder
	line := 1
	startLine := 1
	started := false
	inSingle := false
	dollarTag := ""
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		if c == '\n' {
			line++
		}
		if !started && !isBlank(c) {
			started = true
			startLine = line
		}
		switch {
		case dollarTag != "":
			if strings.HasPrefix(src[i:], dollarTag) {
				cur.WriteString(dollarTag)
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			cur.WriteByte(c)
			i++
		case inSingle:
			cur.WriteByte(c)
			if c == '\'' {
				if i+1 < n && src[i+1] == '\'' {
					cur.WriteByte(src[i+1])
					i += 2
					continue
				}
				inSingle = false
			}
			i++
		case c == '\'':
			inSingle = true
			cur.WriteByte(c)
			i++
		case c == '$':
			if tag, ok := dollarTagAt(src, i); ok {
				dollarTag = tag
				cur.WriteString(tag)
				i += len(tag)
			} else {
				cur.WriteByte(c)
				i++
			}
		case c == ';':
			if text := strings.TrimSpace(cur.String()); text != "" {
				stmts = append(stmts, RawStatement{Text: text, Line: startLine})
			}
			cur.Reset()
			started = false
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	if text := strings.TrimSpace(cur.String()); text != "" {
		stmts = append(stmts, RawStatement{Text: text, Line: startLine})
	}
	return stmts
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// SplitTopLevelCommas splits an ALTER TABLE action list on commas that sit
// outside parens and string literals, so `numeric(10,2)` or a CHECK(...)
// clause's internal commas never split into a bogus extra clause.
func SplitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	inSingle := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case c == '\'':
			inSingle = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}
