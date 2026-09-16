package sqlscan

import (
	"regexp"
	"strings"
)

// TableColumn is a normalized (table, column) identifier pair.
type TableColumn struct{ Table, Column string }

// Ref is a single high-confidence (table, column) reference plus the
// provenance the D-15 cross-reference message needs: which previous-release
// query file and sqlc query name it came from.
type Ref struct{ Table, Column, QueryFile, QueryName string }

// RefSet owns the high/low/params confidence tiers (RESEARCH Pitfall E):
// deterministic-red only on High; Low is informational; Params are never
// asserted as a column of any table.
type RefSet struct {
	High   []Ref
	Low    map[TableColumn]bool
	Params map[string]bool
}

// QueryColumnRefs parses one queries/*.sql file's text into a RefSet.
// schemaCols is SchemaColumns' output and is what makes SELECT * /
// RETURNING * star expansion possible. Low and Params are always non-nil.
func QueryColumnRefs(file, sql string, schemaCols map[string][]string) RefSet {
	r := RefSet{Low: map[TableColumn]bool{}, Params: map[string]bool{}}
	for _, qb := range splitQueryBlocks(sql) {
		extractBlockReferences(&r, file, qb, schemaCols)
	}
	return r
}

// Merge unions other's High/Low/Params into r, lazily initialising r's
// maps so it works against a zero-value receiver.
func (r *RefSet) Merge(other RefSet) {
	r.High = append(r.High, other.High...)
	for k := range other.Low {
		if r.Low == nil {
			r.Low = map[TableColumn]bool{}
		}
		r.Low[k] = true
	}
	for k := range other.Params {
		if r.Params == nil {
			r.Params = map[string]bool{}
		}
		r.Params[k] = true
	}
}

// Lookup reports whether (table, column) appears in the high-confidence set
// and returns the first matching reference for message provenance. Both
// arguments are normalized (the table also schema-stripped), so a
// schema-qualified lookup cannot miss.
func (r RefSet) Lookup(table, column string) (Ref, bool) {
	t := NormalizeIdent(StripSchemaQualifier(table))
	c := NormalizeIdent(column)
	for _, ref := range r.High {
		if ref.Table == t && ref.Column == c {
			return ref, true
		}
	}
	return Ref{}, false
}

// LookupAnyColumn reports whether ANY column of table appears in the
// high-confidence set -- used for DROP TABLE / RENAME TABLE, where "still
// referenced" means the previous release touches the table at all.
func (r RefSet) LookupAnyColumn(table string) (Ref, bool) {
	t := NormalizeIdent(StripSchemaQualifier(table))
	for _, ref := range r.High {
		if ref.Table == t {
			return ref, true
		}
	}
	return Ref{}, false
}

// HasLow reports whether (table, column) appears in the low-confidence set
// -- never deterministic-red, at most an informational note.
func (r RefSet) HasLow(table, column string) bool {
	if r.Low == nil {
		return false
	}
	return r.Low[TableColumn{
		Table:  NormalizeIdent(StripSchemaQualifier(table)),
		Column: NormalizeIdent(column),
	}]
}

func (r *RefSet) addHigh(table, column, file, queryName string) {
	table, column = NormalizeIdent(table), NormalizeIdent(column)
	if table == "" || column == "" {
		return
	}
	r.High = append(r.High, Ref{Table: table, Column: column, QueryFile: file, QueryName: queryName})
}

func (r *RefSet) addLow(table, column string) {
	table, column = NormalizeIdent(table), NormalizeIdent(column)
	if table == "" || column == "" {
		return
	}
	if r.Low == nil {
		r.Low = map[TableColumn]bool{}
	}
	r.Low[TableColumn{Table: table, Column: column}] = true
}

// ---- query block extraction (D-15) ----

// reNameMarker splits a queries/*.sql file into sqlc query blocks on its own
// `-- name: X :kind` marker.
var reNameMarker = regexp.MustCompile(`(?im)^--\s*name:\s*(\S+)\s*:\S+\s*$`)

type queryBlock struct {
	name string
	body string
}

func splitQueryBlocks(raw string) []queryBlock {
	locs := reNameMarker.FindAllStringSubmatchIndex(raw, -1)
	if len(locs) == 0 {
		return nil
	}
	var out []queryBlock
	for i, loc := range locs {
		name := raw[loc[2]:loc[3]]
		start := loc[1]
		end := len(raw)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, queryBlock{name: name, body: raw[start:end]})
	}
	return out
}

var (
	reSqlcNamedParam  = regexp.MustCompile(`sqlc\.(?:arg|narg)\('([^']+)'\)`)
	reAtParam         = regexp.MustCompile(`@([A-Za-z_][A-Za-z0-9_]*)`)
	reFromJoinKeyword = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\b`)
	reIdentWithDot    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*`)
	reIdentSimple     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	reWithCTEName     = regexp.MustCompile(`(?i)(?:\bWITH\b|,)\s*([A-Za-z_][A-Za-z0-9_]*)\s+AS\s*\(`)
	reInsertIntoCols  = regexp.MustCompile(`(?is)\bINSERT\s+INTO\s+([A-Za-z_][A-Za-z0-9_.]*)\s*\(([^)]*)\)`)
	reOnConflictCols  = regexp.MustCompile(`(?is)\bON\s+CONFLICT\s*\(([^)]*)\)`)
	reSelectSeg       = regexp.MustCompile(`(?is)\bSELECT\b(.*?)\bFROM\b`)
	reQualifiedRef    = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
	reQualifiedQuoted = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\."([^"]*)"`)
	reStarQualified   = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.\*`)
	reBareStarSelect  = regexp.MustCompile(`(?i)\bSELECT\s+\*\s+FROM\b`)
	reBareStarReturn  = regexp.MustCompile(`(?i)\bRETURNING\s+\*`)
	reWhereBareCol    = regexp.MustCompile(`(?i)\b(?:WHERE|AND|OR)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|<>|!=|<=|>=|<|>|IS\b|IN\s*\()`)
	reBareSelectItem  = regexp.MustCompile(`(?i)^([A-Za-z_][A-Za-z0-9_]*)(?:\s+AS\s+\S+)?$`)
)

// sqlKeywords is a denylist so the bare-identifier passes never mistake a
// keyword for a column reference. Not exhaustive by design -- an over-broad
// bare-column match here is an unrecoverable D-15 false-red (Pitfall E).
var sqlKeywords = map[string]bool{
	"select": true, "from": true, "where": true, "and": true, "or": true, "not": true,
	"null": true, "is": true, "in": true, "as": true, "on": true, "join": true,
	"left": true, "right": true, "inner": true, "outer": true, "with": true,
	"insert": true, "into": true, "values": true, "update": true, "set": true,
	"delete": true, "returning": true, "order": true, "by": true, "group": true,
	"having": true, "limit": true, "asc": true, "desc": true, "distinct": true,
	"conflict": true, "do": true, "nothing": true, "exists": true, "case": true,
	"when": true, "then": true, "else": true, "end": true, "nulls": true,
	"first": true, "last": true, "for": true, "true": true, "false": true,
	"excluded": true,
}

// nextToken skips leading whitespace and returns the next simple
// identifier-shaped token (no dot) plus the remainder of s after it.
func nextToken(s string) (tok, rest string) {
	s = strings.TrimLeft(s, " \t\n\r")
	tok = reIdentSimple.FindString(s)
	return tok, s[len(tok):]
}

// fromJoinTable is one FROM/JOIN occurrence's table (possibly
// schema-qualified) and its resolved alias, if any.
type fromJoinTable struct {
	table string
	alias string
}

// findFromJoinTables locates every bare FROM/JOIN keyword and tokenizes what
// follows by hand. A single combined "keyword + table + optional alias"
// regex lets the alias group swallow the next clause's own FROM/JOIN
// keyword, making FindAllStringSubmatch skip the real second table -- and
// with it any column referenced only through that table. Matching the bare
// keyword first side-steps that match-consumption trap.
func findFromJoinTables(stripped string) []fromJoinTable {
	var out []fromJoinTable
	for _, loc := range reFromJoinKeyword.FindAllStringIndex(stripped, -1) {
		rest := strings.TrimLeft(stripped[loc[1]:], " \t\n\r")
		table := reIdentWithDot.FindString(rest)
		if table == "" {
			continue
		}
		rest = rest[len(table):]
		tok1, rest2 := nextToken(rest)
		alias := ""
		switch {
		case strings.EqualFold(tok1, "AS"):
			if tok2, _ := nextToken(rest2); tok2 != "" && !sqlKeywords[strings.ToLower(tok2)] {
				alias = tok2
			}
		case tok1 != "" && !sqlKeywords[strings.ToLower(tok1)]:
			alias = tok1
		}
		out = append(out, fromJoinTable{table: table, alias: alias})
	}
	return out
}

// extractParams replaces sqlc.arg('x')/sqlc.narg('x') and @x with an inert
// placeholder (so the alias.col scan never mistakes `sqlc.arg` for a
// qualified reference) and collects the names into a separate bag --
// parameter names are never asserted as columns (RESEARCH D-15 step 10).
func extractParams(body string) (cleaned string, params map[string]bool) {
	params = map[string]bool{}
	body = reSqlcNamedParam.ReplaceAllStringFunc(body, func(m string) string {
		sub := reSqlcNamedParam.FindStringSubmatch(m)
		params[sub[1]] = true
		return " __param_" + sub[1] + " "
	})
	body = reAtParam.ReplaceAllStringFunc(body, func(m string) string {
		sub := reAtParam.FindStringSubmatch(m)
		params[sub[1]] = true
		return " __param_" + sub[1] + " "
	})
	return body, params
}

func extractBlockReferences(r *RefSet, file string, qb queryBlock, schemaCols map[string][]string) {
	body, params := extractParams(qb.body)
	for p := range params {
		if r.Params == nil {
			r.Params = map[string]bool{}
		}
		r.Params[p] = true
	}
	stripped := StripComments(body)

	cteNames := map[string]bool{}
	for _, m := range reWithCTEName.FindAllStringSubmatch(stripped, -1) {
		cteNames[NormalizeIdent(m[1])] = true
	}

	// aliasMap: normalized alias/table-name -> normalized real table, or ""
	// for a CTE alias (deliberately excluded from the real-table set).
	aliasMap := map[string]string{}
	realTables := map[string]bool{}
	for _, fj := range findFromJoinTables(stripped) {
		table := NormalizeIdent(StripSchemaQualifier(fj.table))
		alias := NormalizeIdent(fj.alias)
		if cteNames[table] {
			if alias != "" {
				aliasMap[alias] = ""
			}
			continue
		}
		realTables[table] = true
		aliasMap[table] = table
		if alias != "" {
			aliasMap[alias] = table
		}
	}

	insertTarget := ""
	if m := reInsertIntoCols.FindStringSubmatch(stripped); m != nil {
		insertTarget = NormalizeIdent(StripSchemaQualifier(m[1]))
		aliasMap[insertTarget] = insertTarget
		aliasMap["excluded"] = insertTarget
		for _, col := range strings.Split(m[2], ",") {
			col = strings.TrimSpace(col)
			if col == "" {
				continue
			}
			r.addHigh(insertTarget, StripIdent(col), file, qb.name)
		}
		if m := reOnConflictCols.FindStringSubmatch(stripped); m != nil {
			for _, col := range strings.Split(m[1], ",") {
				col = strings.TrimSpace(col)
				if col == "" {
					continue
				}
				r.addHigh(insertTarget, StripIdent(col), file, qb.name)
			}
		}
	}

	// Qualified alias.col / table.col references: high confidence whenever
	// the alias resolves to a real table.
	for _, m := range reQualifiedRef.FindAllStringSubmatch(stripped, -1) {
		alias := NormalizeIdent(m[1])
		table, ok := aliasMap[alias]
		if !ok || table == "" {
			continue
		}
		r.addHigh(table, m[2], file, qb.name)
	}
	// Same pass, quoted-column form (`a."Mixed"`) -- quotes re-added before
	// normalizing so the byte-exact quoted-identifier rule applies.
	for _, m := range reQualifiedQuoted.FindAllStringSubmatch(stripped, -1) {
		alias := NormalizeIdent(m[1])
		table, ok := aliasMap[alias]
		if !ok || table == "" {
			continue
		}
		r.addHigh(table, `"`+m[2]+`"`, file, qb.name)
	}

	// Star expansion: alias.* and bare * -- bare SELECT * only for a single
	// real table; bare RETURNING * always resolves to the INSERT target.
	for _, m := range reStarQualified.FindAllStringSubmatch(stripped, -1) {
		alias := NormalizeIdent(m[1])
		if table, ok := aliasMap[alias]; ok && table != "" {
			expandStar(r, schemaCols, table, file, qb.name)
		}
	}
	if reBareStarSelect.MatchString(stripped) && len(realTables) == 1 {
		for t := range realTables {
			expandStar(r, schemaCols, t, file, qb.name)
		}
	}
	if reBareStarReturn.MatchString(stripped) && insertTarget != "" {
		expandStar(r, schemaCols, insertTarget, file, qb.name)
	}

	// Bare unqualified columns in WHERE/AND/OR position (also flattens a
	// subquery's own WHERE clause -- B4).
	for _, m := range reWhereBareCol.FindAllStringSubmatch(stripped, -1) {
		classifyBareColumn(r, realTables, m[1], file, qb.name)
	}

	// Bare unqualified explicit SELECT list items.
	if m := reSelectSeg.FindStringSubmatch(stripped); m != nil {
		for _, item := range SplitTopLevelCommas(m[1]) {
			item = strings.TrimSpace(item)
			if item == "" || item == "*" || strings.Contains(item, ".") {
				continue
			}
			if bm := reBareSelectItem.FindStringSubmatch(item); bm != nil {
				classifyBareColumn(r, realTables, bm[1], file, qb.name)
			}
		}
	}
}

// expandStar adds every column of table (per schemaCols) as a
// high-confidence reference. A table missing from schemaCols is silently
// skipped -- no high-confidence claim can be made without it.
func expandStar(r *RefSet, schemaCols map[string][]string, table, file, queryName string) {
	cols, ok := schemaCols[table]
	if !ok {
		return
	}
	for _, c := range cols {
		r.addHigh(table, c, file, queryName)
	}
}

// classifyBareColumn attributes a bare column: high confidence if the query
// has exactly one real table, low confidence (every real table) otherwise
// -- the RESEARCH D-15 conservatism split (Pitfall E).
func classifyBareColumn(r *RefSet, realTables map[string]bool, col, file, queryName string) {
	if sqlKeywords[strings.ToLower(col)] {
		return
	}
	if len(realTables) == 1 {
		for t := range realTables {
			r.addHigh(t, col, file, queryName)
		}
		return
	}
	for t := range realTables {
		r.addLow(t, col)
	}
}
