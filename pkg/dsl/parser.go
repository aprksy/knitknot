package dsl

import (
	"fmt"
	"strconv"
	"strings"
)

type Parser struct {
	l         *Lexer
	curToken  Token
	peekToken Token
	errors    []string
}

func NewParser(input string) *Parser {
	l := NewLexer(input)
	p := &Parser{l: l}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) Parse() (*Query, error) {
	query := &Query{Methods: []*MethodCall{}}

	// fmt.Printf("%s: %s, %s: %s\n", p.curToken.Type, p.curToken.Literal, p.peekToken.Type, p.peekToken.Literal)
	for p.curToken.Type != EOF {
		if p.curToken.Type == Ident {
			method := p.parseMethodCall()
			if method == nil {
				if len(p.errors) > 0 {
					return nil, fmt.Errorf("failed to parse method at pos %d. %s", p.l.position-1, p.errors[0])
				}
				return nil, fmt.Errorf("failed to parse method at pos %d. syntax error", p.l.position-1)
			}
			query.Methods = append(query.Methods, method)
		} else {
			return nil, fmt.Errorf("expected method name, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
		}

		if p.expectPeek(Dot) {
			p.nextToken() // consume dot
		} else {
			break
		}
	}

	if p.curToken.Type == EOF {
		return nil, fmt.Errorf("unexpected end of query at pos %d", p.curToken.PosX)
	}

	return query, nil
}

func (p *Parser) parseMethodCall() *MethodCall {
	methodName := &Identifier{Value: p.curToken.Literal}

	if !p.expectPeek(LParen) {
		return nil
	}
	p.nextToken()

	args := p.parseArguments()
	if args == nil {
		return nil
	}

	methodCall := MethodCall{
		Name:      methodName,
		Arguments: args,
	}
	return &methodCall
}

func (p *Parser) parseArguments() []Expression {
	var args []Expression

	if p.curToken.Type != RParen {
		args = []Expression{}

		arg := p.parseExpression()
		if arg != nil {
			args = append(args, arg)
		}

		for p.peekToken.Type == Comma {
			p.nextToken()
			p.nextToken()
			arg := p.parseExpression()
			if arg != nil {
				args = append(args, arg)
			}
		}
	}

	// if p.peekToken.Type == RParen {
	// 	p.nextToken()
	// 	return args
	// }

	if !p.expectPeek(RParen) {
		return nil
	}

	return args
}

func (p *Parser) parseExpression() Expression {
	switch p.curToken.Type {
	case String:
		return &StringLiteral{Value: p.curToken.Literal}
	case Number:
		if v, err := strconv.Atoi(p.curToken.Literal); err == nil {
			return &NumberLiteral{Value: v}
		}
	}
	p.errors = append(p.errors, fmt.Sprintf("unexpected token: %s", p.curToken.Literal))
	return nil
}

func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken() // advances curToken to peekToken
		return true
	}
	p.errors = append(p.errors, fmt.Sprintf("expected %v, got %v", t, p.peekToken.Type))
	return false
}

// ParseProgram parses one unit: a write statement or a read query.
// Write keywords match case-insensitively (like the REPL); exactly one
// statement per program. Parse (queries only) is unchanged.
func (p *Parser) ParseProgram() (*Program, error) {
	if p.curToken.Type == Ident {
		switch strings.ToUpper(p.curToken.Literal) {
		case WriteAddNode, WriteConnect, WriteUpdate, WriteDelete, WriteDefine:
			w, err := p.parseWriteStatement()
			if err != nil {
				return nil, err
			}
			if p.peekToken.Type != EOF {
				return nil, fmt.Errorf("unexpected trailing input (only one statement per program) at pos %d", p.peekToken.PosX)
			}
			return &Program{Write: w}, nil
		}
	}

	q, err := p.Parse()
	if err != nil {
		return nil, err
	}
	if p.peekToken.Type != EOF {
		return nil, fmt.Errorf("unexpected trailing input (only one statement per program) at pos %d", p.peekToken.PosX)
	}
	return &Program{Query: q}, nil
}

func isWriteVerb(lit string) bool {
	switch strings.ToUpper(lit) {
	case WriteAddNode, WriteConnect, WriteUpdate, WriteDelete, WriteDefine:
		return true
	}
	return false
}

func (p *Parser) parseWriteStatement() (*WriteStatement, error) {
	op := strings.ToUpper(p.curToken.Literal)
	p.nextToken() // cur = first token after the keyword

	switch op {
	case WriteAddNode:
		if p.curToken.Type != Ident {
			return nil, fmt.Errorf("ADDNODE needs a label, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
		}
		label := p.curToken.Literal
		props, err := p.parseProps()
		if err != nil {
			return nil, err
		}
		return &WriteStatement{Op: op, Label: label, Props: props}, nil
	case WriteConnect:
		// CONNECT <from> --<rel> [k=v ...]--> <to> (props inside the arrow,
		// mirroring the REPL; no trailing props).
		edge, props, err := p.parseEdge(true)
		if err != nil {
			return nil, err
		}
		return &WriteStatement{Op: op, Edge: edge, Props: props}, nil
	case WriteUpdate:
		return p.parseUpdate()
	case WriteDelete:
		return p.parseDelete()
	case WriteDefine:
		return p.parseDefine()
	}
	return nil, fmt.Errorf("unknown write statement %q", op)
}

// parseEdge parses <from> --<rel>--> <to>. On entry cur is the from-id;
// on exit cur is the to-id. If inline is true, key=value pairs between
// the rel and --> are collected (CONNECT form); otherwise the edge is
// bare (UPDATE/DELETE form, props trail after the arrow).
func (p *Parser) parseEdge(inline bool) (*EdgeRef, []PropAssign, error) {
	from, err := p.nodeID("edge endpoint")
	if err != nil {
		return nil, nil, err
	}
	if !p.expectPeek(DashDash) {
		return nil, nil, fmt.Errorf("expected -- after %q, got %v at pos %d", from, p.peekToken.Type, p.peekToken.PosX)
	}
	p.nextToken() // cur = rel
	if p.curToken.Type != Ident {
		return nil, nil, fmt.Errorf("expected relationship type, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
	}
	rel := p.curToken.Literal
	var props []PropAssign
	if inline {
		props, err = p.parseProps()
		if err != nil {
			return nil, nil, err
		}
	}
	if !p.expectPeek(Arrow) {
		return nil, nil, fmt.Errorf("expected --> to close the edge, got %v at pos %d", p.peekToken.Type, p.peekToken.PosX)
	}
	p.nextToken() // cur = to-id
	to, err := p.nodeID("edge endpoint")
	if err != nil {
		return nil, nil, err
	}
	return &EdgeRef{From: from, Rel: rel, To: to}, props, nil
}

func (p *Parser) parseUpdate() (*WriteStatement, error) {
	if p.curToken.Type != Ident {
		return nil, fmt.Errorf("UPDATE needs NODE or EDGE, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
	}
	switch strings.ToUpper(p.curToken.Literal) {
	case "NODE":
		p.nextToken() // cur = id
		id, err := p.nodeID("node id")
		if err != nil {
			return nil, err
		}
		props, err := p.parseProps()
		if err != nil {
			return nil, err
		}
		return &WriteStatement{Op: WriteUpdate, NodeID: id, Props: props}, nil
	case "EDGE":
		p.nextToken() // cur = from-id
		edge, _, err := p.parseEdge(false)
		if err != nil {
			return nil, err
		}
		props, err := p.parseProps()
		if err != nil {
			return nil, err
		}
		return &WriteStatement{Op: WriteUpdate, Edge: edge, Props: props}, nil
	}
	return nil, fmt.Errorf("UPDATE needs NODE or EDGE, got %q at pos %d", p.curToken.Literal, p.curToken.PosX)
}

func (p *Parser) parseDelete() (*WriteStatement, error) {
	if p.curToken.Type != Ident {
		return nil, fmt.Errorf("DELETE needs NODE or EDGE, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
	}
	switch strings.ToUpper(p.curToken.Literal) {
	case "NODE":
		p.nextToken() // cur = id
		id, err := p.nodeID("node id")
		if err != nil {
			return nil, err
		}
		return &WriteStatement{Op: WriteDelete, NodeID: id}, nil
	case "EDGE":
		p.nextToken() // cur = from-id
		edge, _, err := p.parseEdge(false)
		if err != nil {
			return nil, err
		}
		return &WriteStatement{Op: WriteDelete, Edge: edge}, nil
	}
	return nil, fmt.Errorf("DELETE needs NODE or EDGE, got %q at pos %d", p.curToken.Literal, p.curToken.PosX)
}

func (p *Parser) parseDefine() (*WriteStatement, error) {
	// DEFINE <verb> TO <Label> VIA <prop> (TO/VIA case-insensitive, like the REPL).
	if p.curToken.Type != Ident {
		return nil, fmt.Errorf("DEFINE needs a verb name, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
	}
	verb := p.curToken.Literal
	p.nextToken()
	if p.curToken.Type != Ident || strings.ToUpper(p.curToken.Literal) != "TO" {
		return nil, fmt.Errorf("DEFINE needs TO after the verb, got %q at pos %d", p.curToken.Literal, p.curToken.PosX)
	}
	p.nextToken()
	if p.curToken.Type != Ident {
		return nil, fmt.Errorf("DEFINE needs a label after TO, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
	}
	label := p.curToken.Literal
	p.nextToken()
	if p.curToken.Type != Ident || strings.ToUpper(p.curToken.Literal) != "VIA" {
		return nil, fmt.Errorf("DEFINE needs VIA after the label, got %q at pos %d", p.curToken.Literal, p.curToken.PosX)
	}
	p.nextToken()
	if p.curToken.Type != Ident {
		return nil, fmt.Errorf("DEFINE needs a property after VIA, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
	}
	return &WriteStatement{Op: WriteDefine, VerbName: verb, TargetLabel: label, MatchOn: p.curToken.Literal}, nil
}

// nodeID reads one endpoint/id: a bare word, a quoted string, or a number.
// The REPL accepts any raw string as an id; the grammar asks multi-word or
// odd ids to be quoted.
func (p *Parser) nodeID(what string) (string, error) {
	switch p.curToken.Type {
	case Ident, String, Number:
		return p.curToken.Literal, nil
	}
	return "", fmt.Errorf("expected %s, got %v at pos %d", what, p.curToken.Type, p.curToken.PosX)
}

// parseProps parses trailing key=value pairs. It stops before EOF, before
// any non-Ident, and before anything that opens a new statement, so
// "ADDNODE A ADDNODE B" fails as two statements instead of misreading the
// second ADDNODE as a property key.
func (p *Parser) parseProps() ([]PropAssign, error) {
	var props []PropAssign
	for p.peekToken.Type == Ident && !isWriteVerb(p.peekToken.Literal) {
		p.nextToken() // cur = key
		key := p.curToken.Literal
		if !p.expectPeek(Assign) {
			return nil, fmt.Errorf("expected = after property %q at pos %d", key, p.peekToken.PosX)
		}
		p.nextToken() // cur = value
		val, err := p.parsePropValue()
		if err != nil {
			return nil, err
		}
		props = append(props, PropAssign{Key: key, Value: val})
	}
	return props, nil
}

// parsePropValue reads one property value: 'quoted', 12, -5, or a bare
// word (kept as a string, like the REPL's parseProps).
func (p *Parser) parsePropValue() (Expression, error) {
	switch p.curToken.Type {
	case String:
		return &StringLiteral{Value: p.curToken.Literal}, nil
	case Number:
		if v, err := strconv.Atoi(p.curToken.Literal); err == nil {
			return &NumberLiteral{Value: v}, nil
		}
		return nil, fmt.Errorf("bad number %q at pos %d", p.curToken.Literal, p.curToken.PosX)
	case Ident:
		return &StringLiteral{Value: p.curToken.Literal}, nil
	}
	return nil, fmt.Errorf("expected property value, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
}
