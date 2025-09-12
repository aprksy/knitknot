package dsl

import (
	"fmt"
	"strconv"
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

	for p.curToken.Type != EOF {
		if p.curToken.Type == Ident {
			method := p.parseMethodCall()
			if method != nil {
				query.Methods = append(query.Methods, method)
			} else {
				break
			}
		}

		if p.peekToken.Type == Dot {
			p.nextToken() // consume dot
			p.nextToken()
		} else {
			break
		}
	}

	if len(p.errors) > 0 {
		return nil, fmt.Errorf("parser errors: %v", p.errors)
	}

	return query, nil
}

func (p *Parser) parseMethodCall() *MethodCall {
	methodName := &Identifier{Value: p.curToken.Literal}
	p.nextToken()

	if !p.expectPeek(Lparen) {
		return nil
	}
	p.nextToken()

	args := p.parseArguments()
	if args == nil {
		return nil
	}

	if !p.expectPeek(Rparen) {
		return nil
	}
	p.nextToken()

	return &MethodCall{
		Name:      methodName,
		Arguments: args,
	}
}

func (p *Parser) parseArguments() []Expression {
	var args []Expression

	if p.peekToken.Type == Rparen {
		p.nextToken()
		return args
	}

	p.nextToken()
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
	case Ident:
		return &Identifier{Value: p.curToken.Literal}
	}
	p.errors = append(p.errors, fmt.Sprintf("unexpected token: %s", p.curToken.Literal))
	return nil
}

func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	} else {
		p.errors = append(p.errors, fmt.Sprintf("expected next token to be %s, got %s", t, p.peekToken.Type))
		return false
	}
}
