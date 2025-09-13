package dsl

// TokenType represents a lexical token
type TokenType string

const (
	Illegal TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"
	Ident   TokenType = "IDENT"
	Int     TokenType = "INT"
	String  TokenType = "STRING"
	Number  TokenType = "NUMBER"

	Assign    TokenType = "ASSIGN"
	Plus      TokenType = "PLUS"
	Comma     TokenType = "COMMA"
	Semicolon TokenType = "SEMICOLON"

	LParen TokenType = "LPAREN"
	RParen TokenType = "RPAREN"
	Dot    TokenType = "DOT"
)

// Token represents a lexical token
type Token struct {
	Type    TokenType
	Literal string
	PosX    int
}
