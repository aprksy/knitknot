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

	// fmt.Printf("%s: %s, %s: %s\n", p.curToken.Type, p.curToken.Literal, p.peekToken.Type, p.peekToken.Literal)
	for p.curToken.Type != EOF {
		if p.curToken.Type == Ident {
			method := p.parseMethodCall()
			if method == nil {
				return nil, fmt.Errorf("failed to parse method at pos %d. %s", p.l.position-1, p.errors[0])
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
		return nil, fmt.Errorf("expected method name, got %v at pos %d", p.curToken.Type, p.curToken.PosX)
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
