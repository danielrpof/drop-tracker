package sqlscan_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/sqlscan"
)

func TestParse_DropTableNormalizesAndKeepsRaw(t *testing.T) {
	stmts := sqlscan.Parse("DROP TABLE public.events;")
	if len(stmts) != 1 {
		t.Fatalf("Parse produced %d statements, want 1", len(stmts))
	}
	dt, ok := stmts[0].(sqlscan.DropTable)
	if !ok {
		t.Fatalf("Parse[0] = %T, want sqlscan.DropTable", stmts[0])
	}
	if dt.Name != "events" || dt.RawName != "public.events" || dt.Line != 1 {
		t.Fatalf("DropTable = %+v, want {Line:1 Name:events RawName:public.events}", dt)
	}
}

func TestParse_AlterTableActionsInClauseOrder(t *testing.T) {
	stmts := sqlscan.Parse("ALTER TABLE events DROP COLUMN a, ADD COLUMN b text, ALTER COLUMN c TYPE bigint;")
	if len(stmts) != 1 {
		t.Fatalf("Parse produced %d statements, want 1", len(stmts))
	}
	at, ok := stmts[0].(sqlscan.AlterTable)
	if !ok {
		t.Fatalf("Parse[0] = %T, want sqlscan.AlterTable", stmts[0])
	}
	if at.Name != "events" {
		t.Fatalf("AlterTable.Name = %q, want events", at.Name)
	}
	if len(at.Actions) != 3 {
		t.Fatalf("AlterTable.Actions = %#v, want 3 in clause order", at.Actions)
	}
	if _, ok := at.Actions[0].(sqlscan.DropColumn); !ok {
		t.Fatalf("Actions[0] = %T, want DropColumn", at.Actions[0])
	}
	if _, ok := at.Actions[1].(sqlscan.AddColumn); !ok {
		t.Fatalf("Actions[1] = %T, want AddColumn", at.Actions[1])
	}
	if _, ok := at.Actions[2].(sqlscan.AlterColumnType); !ok {
		t.Fatalf("Actions[2] = %T, want AlterColumnType", at.Actions[2])
	}
}

func TestParse_EachActionTypeFromItsClauseForm(t *testing.T) {
	cases := []struct {
		clause string
		want   any
	}{
		{"DROP COLUMN release_type", sqlscan.DropColumn{}},
		{"RENAME TO events_old", sqlscan.RenameTable{}},
		{"RENAME COLUMN old_name TO new_name", sqlscan.RenameColumn{}},
		{"ALTER COLUMN c TYPE bigint", sqlscan.AlterColumnType{}},
		{"ALTER COLUMN c SET NOT NULL", sqlscan.SetNotNull{}},
		{"ADD CONSTRAINT x CHECK (c > 0)", sqlscan.AddCheck{}},
		{"ADD COLUMN d text", sqlscan.AddColumn{}},
	}
	for _, tc := range cases {
		t.Run(tc.clause, func(t *testing.T) {
			stmts := sqlscan.Parse("ALTER TABLE events " + tc.clause + ";")
			at := stmts[0].(sqlscan.AlterTable)
			if len(at.Actions) != 1 {
				t.Fatalf("Actions = %#v, want exactly 1", at.Actions)
			}
			if got := reflect.TypeOf(at.Actions[0]); got != reflect.TypeOf(tc.want) {
				t.Fatalf("Actions[0] = %v, want %v", got, reflect.TypeOf(tc.want))
			}
		})
	}
}

func TestParse_AddCheckNeverAddColumn(t *testing.T) {
	at := sqlscan.Parse("ALTER TABLE events ADD CONSTRAINT x CHECK (c > 0);")[0].(sqlscan.AlterTable)
	if _, ok := at.Actions[0].(sqlscan.AddCheck); !ok {
		t.Fatalf("ADD CONSTRAINT ... CHECK classified as %T, want AddCheck", at.Actions[0])
	}
}

func TestParse_AddColumnNotNullAndDefaultAreIndependent(t *testing.T) {
	at := sqlscan.Parse("ALTER TABLE events ADD COLUMN n text NOT NULL DEFAULT 'x';")[0].(sqlscan.AlterTable)
	ac := at.Actions[0].(sqlscan.AddColumn)
	if !ac.NotNull || !ac.HasDefault {
		t.Fatalf("AddColumn = %+v, want NotNull=true HasDefault=true", ac)
	}

	at2 := sqlscan.Parse("ALTER TABLE events ADD COLUMN n text NOT NULL;")[0].(sqlscan.AlterTable)
	ac2 := at2.Actions[0].(sqlscan.AddColumn)
	if !ac2.NotNull || ac2.HasDefault {
		t.Fatalf("AddColumn = %+v, want NotNull=true HasDefault=false", ac2)
	}
}

func TestParse_RenameColumnCarriesFromAndToSeparately(t *testing.T) {
	at := sqlscan.Parse(`ALTER TABLE events RENAME COLUMN old_name TO new_col;`)[0].(sqlscan.AlterTable)
	rc := at.Actions[0].(sqlscan.RenameColumn)
	if rc.From != "old_name" || rc.To != "new_col" || rc.RawFrom != "old_name" || rc.RawTo != "new_col" {
		t.Fatalf("RenameColumn = %+v, want From/RawFrom=old_name To/RawTo=new_col", rc)
	}
	// A double-quoted DDL identifier is stripped of its quotes by StripIdent
	// before NormalizeIdent, so it folds -- preserved-as-is asymmetry.
	at2 := sqlscan.Parse(`ALTER TABLE events RENAME COLUMN "Old" TO new_col;`)[0].(sqlscan.AlterTable)
	rc2 := at2.Actions[0].(sqlscan.RenameColumn)
	if rc2.From != "old" || rc2.RawFrom != "Old" {
		t.Fatalf("RenameColumn = %+v, want From=old RawFrom=Old", rc2)
	}
}

func TestParse_IsTotal(t *testing.T) {
	stmts := sqlscan.Parse("CREATE INDEX idx_events_artist ON events (artist_id);")
	if len(stmts) != 1 {
		t.Fatalf("Parse produced %d statements, want 1", len(stmts))
	}
	if _, ok := stmts[0].(sqlscan.RawStatement); !ok {
		t.Fatalf("Parse[0] = %T, want sqlscan.RawStatement", stmts[0])
	}
	if got := sqlscan.Parse(""); len(got) != 0 {
		t.Fatalf("Parse(\"\") = %#v, want empty", got)
	}
}

func TestParse_CreateTableColumnsExcludeConstraintClauses(t *testing.T) {
	sql := `CREATE TABLE t (
	    id bigserial PRIMARY KEY,
	    a text NOT NULL,
	    b int,
	    CONSTRAINT t_a_check CHECK (a <> ''),
	    UNIQUE (b),
	    FOREIGN KEY (a) REFERENCES other(x)
	);`
	ct := sqlscan.Parse(sql)[0].(sqlscan.CreateTable)
	got := append([]string(nil), ct.Columns...)
	sort.Strings(got)
	want := []string{"a", "b", "id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CreateTable.Columns = %#v, want %#v", got, want)
	}
}

func TestSchemaColumns(t *testing.T) {
	sql := `CREATE TABLE artists (
    id BIGSERIAL PRIMARY KEY,
    mbid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    CONSTRAINT artists_name_check CHECK (name <> '')
);
ALTER TABLE artists ADD COLUMN image_url TEXT;
`
	cols := sqlscan.SchemaColumns(sqlscan.Parse(sql))
	got := cols["artists"]
	want := map[string]bool{"id": true, "mbid": true, "name": true, "image_url": true}
	if len(got) != len(want) {
		t.Fatalf("SchemaColumns() artists columns = %#v, want exactly %v", got, want)
	}
	for _, c := range got {
		if !want[c] {
			t.Fatalf("SchemaColumns() produced unexpected column %q (CONSTRAINT clause leaked through?)", c)
		}
	}
}
