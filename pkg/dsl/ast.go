package dsl

// Node is a node in the AST
type Node interface {
	TokenLiteral() string
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
