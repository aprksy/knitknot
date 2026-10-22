package dsl

// Node is a node in the AST
type Node interface {
	TokenLiteral() string
}

// Program is one parsed unit: either a read query or a write statement.
// W1 parses both; execution of writes lands in W2.
type Program struct {
	Query *Query
	Write *WriteStatement
}

func (p *Program) TokenLiteral() string {
	if p.Write != nil {
		return p.Write.Op
	}
	if p.Query != nil {
		return p.Query.TokenLiteral()
	}
	return ""
}

// Write statement verbs (canonical upper-case; matched case-insensitively).
const (
	WriteAddNode = "ADDNODE"
	WriteConnect = "CONNECT"
	WriteUpdate  = "UPDATE"
	WriteDelete  = "DELETE"
	WriteDefine  = "DEFINE"
)

// WriteStatement is one mutation: ADDNODE / CONNECT / UPDATE / DELETE / DEFINE.
// Only the fields for the given Op are set.
type WriteStatement struct {
	Op string
	// ADDNODE
	Label string
	// UPDATE / DELETE target: exactly one of NodeID / Edge is set.
	NodeID string
	Edge   *EdgeRef
	// ADDNODE / CONNECT / UPDATE props, in source order.
	Props []PropAssign
	// DEFINE <VerbName> TO <TargetLabel> VIA <MatchOn>
	VerbName    string
	TargetLabel string
	MatchOn     string
}

func (w *WriteStatement) TokenLiteral() string { return w.Op }

// EdgeRef is one edge endpoint pair: <From> --<Rel>--> <To>.
type EdgeRef struct {
	From string
	Rel  string
	To   string
}

func (e *EdgeRef) TokenLiteral() string { return e.Rel }

// PropAssign is one key=value property. Value is a StringLiteral,
// NumberLiteral, or bare-word Ident kept as a string.
type PropAssign struct {
	Key   string
	Value Expression
}

// Query represents the full chain
type Query struct {
	Methods []*MethodCall
}

func (q *Query) TokenLiteral() string {
	if len(q.Methods) > 0 {
		return q.Methods[0].TokenLiteral()
	}
	return ""
}

// MethodCall represents a single call: Find('User')
type MethodCall struct {
	Name      *Identifier
	Arguments []Expression
}

func (m *MethodCall) TokenLiteral() string { return m.Name.Value }
func (m *MethodCall) ExpressionNode()      {}

// Identifier: Find, Has, Where
type Identifier struct {
	Value string
}

func (i *Identifier) ExpressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Value }

// Expressions
type Expression interface {
	ExpressionNode()
	TokenLiteral() string
}

// StringLiteral: 'User'
type StringLiteral struct {
	Value string
}

func (s *StringLiteral) ExpressionNode()      {}
func (s *StringLiteral) TokenLiteral() string { return s.Value }

// NumberLiteral: 30
type NumberLiteral struct {
	Value int
}

func (n *NumberLiteral) ExpressionNode()      {}
func (n *NumberLiteral) TokenLiteral() string { return string(rune(n.Value)) } // not ideal, just for now
