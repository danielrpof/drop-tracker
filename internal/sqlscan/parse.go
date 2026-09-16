package sqlscan

import (
	"regexp"
	"strings"
)

// Statement is the sealed result type of Parse. Parse is TOTAL: it never
// returns an error and never drops a statement -- anything it does not
// structurally recognise comes back as RawStatement.
type Statement interface{ isStatement() }

// CreateTable is a `CREATE TABLE` statement with its column names already
// filtered of constraint clauses.
type CreateTable struct {
	Line    int
	Name    string   // normalized + schema-stripped
	RawName string   // as written (post-StripIdent), for message rendering only
	Columns []string // normalized column names
}

// AlterTable is an `ALTER TABLE` statement; one recognised clause yields at
// most one Action, in clause order.
type AlterTable struct {
	Line    int
	Name    string
	RawName string
	Actions []Action
}

// DropTable is a `DROP TABLE` statement.
type DropTable struct {
	Line    int
	Name    string
	RawName string
}

func (CreateTable) isStatement()  {}
func (AlterTable) isStatement()   {}
func (DropTable) isStatement()    {}
func (RawStatement) isStatement() {}

// Action is a sealed ALTER TABLE clause. The clause form is matched in the
// precedence the old classifyAlterClause switch used: DropColumn ->
// RenameTable -> RenameColumn -> AlterColumnType -> SetNotNull -> AddCheck
// -> AddColumn. A clause matching none of them yields no Action (mirroring
// today's "no finding" outcome).
type Action interface{ isAction() }

type DropColumn struct{ Column, RawColumn string }
type RenameColumn struct{ From, To, RawFrom, RawTo string }
type AddColumn struct {
	Column, RawColumn   string
	NotNull, HasDefault bool
}
type AlterColumnType struct{ Column, RawColumn string }
type SetNotNull struct{ Column, RawColumn string }
type AddCheck struct{}
type RenameTable struct{ To, RawTo string }

func (DropColumn) isAction()      {}
func (RenameColumn) isAction()    {}
func (AddColumn) isAction()       {}
func (AlterColumnType) isAction() {}
func (SetNotNull) isAction()      {}
func (AddCheck) isAction()        {}
func (RenameTable) isAction()     {}

// DDL pattern set (moved verbatim from cmd/migration-check, D-08).
var (
	reDropTable   = regexp.MustCompile(`(?is)^DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\S+)`)
	reAlterTable  = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\S+)\s+(.*)$`)
	reCreateTable = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s+(\S+)\s*\((.*)\)\s*$`)
	reDropColumn  = regexp.MustCompile(`(?is)^DROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?(\S+)`)
	reRenameTbl   = regexp.MustCompile(`(?is)^RENAME\s+TO\s+(\S+)`)
	reRenameCol   = regexp.MustCompile(`(?is)^RENAME\s+(?:COLUMN\s+)?(\S+)\s+TO\s+(\S+)`)
	reAlterType   = regexp.MustCompile(`(?is)^ALTER\s+COLUMN\s+(\S+)\s+(?:SET\s+DATA\s+)?TYPE\b`)
	reSetNotNull  = regexp.MustCompile(`(?is)^ALTER\s+COLUMN\s+(\S+)\s+SET\s+NOT\s+NULL\b`)
	reAddCheck    = regexp.MustCompile(`(?is)^ADD\s+(?:CONSTRAINT\s+\S+\s+)?CHECK\s*\(`)
	reAddColumn   = regexp.MustCompile(`(?is)^ADD\s+(?:COLUMN\s+)?(\S+)\s`)
	reNotNull     = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
	reDefault     = regexp.MustCompile(`(?i)\bDEFAULT\b`)
)

// tableDefKeywords are CREATE TABLE clause prefixes that are constraints,
// not column definitions, so their leading token is never taken as a
// column name.
var tableDefKeywords = []string{"CONSTRAINT", "PRIMARY KEY", "UNIQUE", "CHECK", "FOREIGN KEY"}

// Parse lexes sql (StripComments then SplitStatements) and classifies each
// statement into the sealed Statement set. It is total -- an unrecognised
// statement is returned as its RawStatement.
func Parse(sql string) []Statement {
	var out []Statement
	for _, st := range SplitStatements(StripComments(sql)) {
		text := strings.TrimSpace(st.Text)
		switch {
		case reDropTable.MatchString(text):
			m := reDropTable.FindStringSubmatch(text)
			raw := StripIdent(m[1])
			out = append(out, DropTable{
				Line: st.Line, Name: NormalizeIdent(StripSchemaQualifier(raw)), RawName: raw,
			})
		case reAlterTable.MatchString(text):
			m := reAlterTable.FindStringSubmatch(text)
			raw := StripIdent(m[1])
			at := AlterTable{
				Line: st.Line, Name: NormalizeIdent(StripSchemaQualifier(raw)), RawName: raw,
			}
			for _, clause := range SplitTopLevelCommas(m[2]) {
				clause = strings.TrimSpace(clause)
				if clause == "" {
					continue
				}
				if a := parseAlterAction(clause); a != nil {
					at.Actions = append(at.Actions, a)
				}
			}
			out = append(out, at)
		case reCreateTable.MatchString(text):
			m := reCreateTable.FindStringSubmatch(text)
			raw := StripIdent(m[1])
			out = append(out, CreateTable{
				Line:    st.Line,
				Name:    NormalizeIdent(StripSchemaQualifier(raw)),
				RawName: raw,
				Columns: createTableColumns(m[2]),
			})
		default:
			out = append(out, st)
		}
	}
	return out
}

func createTableColumns(inner string) []string {
	var cols []string
	for _, colDef := range SplitTopLevelCommas(inner) {
		colDef = strings.TrimSpace(colDef)
		if colDef == "" {
			continue
		}
		upper := strings.ToUpper(colDef)
		isConstraint := false
		for _, kw := range tableDefKeywords {
			if strings.HasPrefix(upper, kw) {
				isConstraint = true
				break
			}
		}
		if isConstraint {
			continue
		}
		fields := strings.Fields(colDef)
		if len(fields) == 0 {
			continue
		}
		cols = append(cols, NormalizeIdent(StripIdent(fields[0])))
	}
	return cols
}

// parseAlterAction maps one ALTER TABLE clause to at most one Action using
// the locked precedence order.
func parseAlterAction(clause string) Action {
	switch {
	case reDropColumn.MatchString(clause):
		raw := StripIdent(reDropColumn.FindStringSubmatch(clause)[1])
		return DropColumn{Column: NormalizeIdent(raw), RawColumn: raw}
	case reRenameTbl.MatchString(clause):
		raw := StripIdent(reRenameTbl.FindStringSubmatch(clause)[1])
		return RenameTable{To: NormalizeIdent(StripSchemaQualifier(raw)), RawTo: raw}
	case reRenameCol.MatchString(clause):
		m := reRenameCol.FindStringSubmatch(clause)
		rf, rt := StripIdent(m[1]), StripIdent(m[2])
		return RenameColumn{From: NormalizeIdent(rf), To: NormalizeIdent(rt), RawFrom: rf, RawTo: rt}
	case reAlterType.MatchString(clause):
		raw := StripIdent(reAlterType.FindStringSubmatch(clause)[1])
		return AlterColumnType{Column: NormalizeIdent(raw), RawColumn: raw}
	case reSetNotNull.MatchString(clause):
		raw := StripIdent(reSetNotNull.FindStringSubmatch(clause)[1])
		return SetNotNull{Column: NormalizeIdent(raw), RawColumn: raw}
	case reAddCheck.MatchString(clause):
		return AddCheck{}
	case reAddColumn.MatchString(clause):
		raw := StripIdent(reAddColumn.FindStringSubmatch(clause)[1])
		return AddColumn{
			Column:     NormalizeIdent(raw),
			RawColumn:  raw,
			NotNull:    reNotNull.MatchString(clause),
			HasDefault: reDefault.MatchString(clause),
		}
	}
	return nil
}

// SchemaColumns builds the "all columns of table X" set used to expand
// SELECT * / RETURNING *, reading CreateTable.Columns and AddColumn actions
// only. Keyed by normalized table name.
func SchemaColumns(stmts []Statement) map[string][]string {
	cols := map[string][]string{}
	for _, s := range stmts {
		switch st := s.(type) {
		case CreateTable:
			cols[st.Name] = append(cols[st.Name], st.Columns...)
		case AlterTable:
			for _, a := range st.Actions {
				if ac, ok := a.(AddColumn); ok {
					cols[st.Name] = append(cols[st.Name], ac.Column)
				}
			}
		}
	}
	return cols
}
