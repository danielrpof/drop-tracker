package sqlscan

import "strings"

// NormalizeIdent folds an unquoted SQL identifier to lower case (Postgres's
// own unquoted-identifier rule) and preserves a double-quoted identifier
// byte-exact (quotes included), so both sides of a D-15 comparison agree
// regardless of how the identifier was written.
func NormalizeIdent(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s
	}
	return strings.ToLower(s)
}

// StripSchemaQualifier drops a leading `schema.` qualifier defensively
// (RESEARCH blind spot B6 -- `public.events` resolves to table `events`).
func StripSchemaQualifier(table string) string {
	table = strings.TrimSpace(table)
	if idx := strings.LastIndex(table, "."); idx >= 0 {
		return table[idx+1:]
	}
	return table
}

// StripIdent trims surrounding double quotes and trailing punctuation a
// regex capture group may include at a clause boundary.
func StripIdent(s string) string {
	s = strings.Trim(s, `"`)
	return strings.TrimRight(s, ";,()")
}
