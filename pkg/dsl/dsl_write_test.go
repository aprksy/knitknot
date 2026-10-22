package dsl_test

// W1 golden tests: write statements parse to the expected AST.
// Execution lands in W2; these tests pin the grammar only.

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aprksy/knitknot/pkg/dsl"
)

func TestParseWriteProgram(t *testing.T) {
	str := func(v string) *dsl.StringLiteral { return &dsl.StringLiteral{Value: v} }
	num := func(v int) *dsl.NumberLiteral { return &dsl.NumberLiteral{Value: v} }

	cases := []struct {
		name  string
		input string
		want  *dsl.WriteStatement
	}{
		{
			"addnode with props",
			"ADDNODE Person name=Alice age=35",
			&dsl.WriteStatement{Op: dsl.WriteAddNode, Label: "Person", Props: []dsl.PropAssign{
				{Key: "name", Value: str("Alice")},
				{Key: "age", Value: num(35)},
			}},
		},
		{
			"addnode bare",
			"ADDNODE Person",
			&dsl.WriteStatement{Op: dsl.WriteAddNode, Label: "Person"},
		},
		{
			"addnode lowercase keyword and quoted value",
			"addnode Person name='Alice Smith'",
			&dsl.WriteStatement{Op: dsl.WriteAddNode, Label: "Person", Props: []dsl.PropAssign{
				{Key: "name", Value: str("Alice Smith")},
			}},
		},
		{
			"addnode negative number",
			"ADDNODE Person score=-5",
			&dsl.WriteStatement{Op: dsl.WriteAddNode, Label: "Person", Props: []dsl.PropAssign{
				{Key: "score", Value: num(-5)},
			}},
		},
		{
			"connect bare",
			"CONNECT n1 --knows--> n2",
			&dsl.WriteStatement{Op: dsl.WriteConnect, Edge: &dsl.EdgeRef{From: "n1", Rel: "knows", To: "n2"}},
		},
		{
			"connect with inline props",
			"CONNECT n1 --knows since=2020--> n2",
			&dsl.WriteStatement{Op: dsl.WriteConnect, Edge: &dsl.EdgeRef{From: "n1", Rel: "knows", To: "n2"}, Props: []dsl.PropAssign{
				{Key: "since", Value: num(2020)},
			}},
		},
		{
			"update node",
			"UPDATE NODE n1 age=36 city=X",
			&dsl.WriteStatement{Op: dsl.WriteUpdate, NodeID: "n1", Props: []dsl.PropAssign{
				{Key: "age", Value: num(36)},
				{Key: "city", Value: str("X")},
			}},
		},
		{
			"update edge with trailing props",
			"UPDATE EDGE n1 --knows--> n2 since=2021",
			&dsl.WriteStatement{Op: dsl.WriteUpdate, Edge: &dsl.EdgeRef{From: "n1", Rel: "knows", To: "n2"}, Props: []dsl.PropAssign{
				{Key: "since", Value: num(2021)},
			}},
		},
		{
			"delete node",
			"DELETE NODE n1",
			&dsl.WriteStatement{Op: dsl.WriteDelete, NodeID: "n1"},
		},
		{
			"delete edge",
			"DELETE EDGE n1 --knows--> n2",
			&dsl.WriteStatement{Op: dsl.WriteDelete, Edge: &dsl.EdgeRef{From: "n1", Rel: "knows", To: "n2"}},
		},
		{
			"define",
			"DEFINE has_skill TO Skill VIA name",
			&dsl.WriteStatement{Op: dsl.WriteDefine, VerbName: "has_skill", TargetLabel: "Skill", MatchOn: "name"},
		},
		{
			"define lowercase",
			"define has_skill to Skill via name",
			&dsl.WriteStatement{Op: dsl.WriteDefine, VerbName: "has_skill", TargetLabel: "Skill", MatchOn: "name"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := dsl.NewParser(tc.input).ParseProgram()
			if err != nil {
				t.Fatalf("ParseProgram(%q) error: %v", tc.input, err)
			}
			if prog.Query != nil {
				t.Fatalf("ParseProgram(%q) parsed as query, want write", tc.input)
			}
			if prog.Write == nil {
				t.Fatalf("ParseProgram(%q) has no write statement", tc.input)
			}
			if !reflect.DeepEqual(prog.Write, tc.want) {
				t.Fatalf("ParseProgram(%q)\n got: %#v\nwant: %#v", tc.input, prog.Write, tc.want)
			}
		})
	}
}

func TestParseWriteProgramErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string // substring of the error
	}{
		{"empty", "", "error"},
		{"addnode missing label", "ADDNODE", "needs a label"},
		{"prop missing value", "ADDNODE Person name=", "expected property value"},
		{"prop missing equals", "ADDNODE Person name", "expected ="},
		{"two statements", "ADDNODE Person age=1 ADDNODE Group", "only one statement"},
		{"connect missing rel", "CONNECT n1 --> n2", "expected --"},
		{"connect missing target", "CONNECT n1 --rel-->", "expected edge endpoint"},
		{"connect trailing props rejected", "CONNECT n1 --rel--> n2 extra=1", "only one statement"},
		{"update bad target", "UPDATE FOO n1", "needs NODE or EDGE"},
		{"update node missing id", "UPDATE NODE", "expected node id"},
		{"delete node missing id", "DELETE NODE", "expected node id"},
		{"define missing to", "DEFINE has_skill Skill name", "needs TO"},
		{"define missing via", "DEFINE has_skill TO Skill", "needs VIA"},
		{"trailing garbage after query", "Find('x') garbage", "unexpected trailing input"},
		{"broken query still errors", "Find('x'", "error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dsl.NewParser(tc.input).ParseProgram()
			if err == nil {
				t.Fatalf("ParseProgram(%q) succeeded, want error containing %q", tc.input, tc.want)
			}
			if tc.want != "error" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ParseProgram(%q) error = %v, want substring %q", tc.input, err, tc.want)
			}
		})
	}
}

func TestParseProgramQueryStillWorks(t *testing.T) {
	// The read path through ParseProgram must match Parse.
	prog, err := dsl.NewParser("Find('User').Where('n.age', '>', 30).Limit(10)").ParseProgram()
	if err != nil {
		t.Fatalf("ParseProgram(query) error: %v", err)
	}
	if prog.Write != nil {
		t.Fatalf("query parsed as write: %#v", prog.Write)
	}
	if prog.Query == nil || len(prog.Query.Methods) != 3 {
		t.Fatalf("want 3-method query, got %#v", prog.Query)
	}
	if got := prog.Query.Methods[0].TokenLiteral(); got != "Find" {
		t.Fatalf("first method = %q, want Find", got)
	}
}
