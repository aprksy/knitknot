package dsl

// TokenType represents a lexical token
type TokenType string

const (
	Illegal TokenType = "ILLEGAL"
	EOF     TokenType = "EOF"

	// Symbols
	Dot    TokenType = "."
	Lparen TokenType = "("
	Rparen TokenType = ")"
	Comma  TokenType = ","

	// Literals
	Ident  TokenType = "IDENT"  // Find, Has, Where
	String TokenType = "STRING" // 'User', 'Go'
	Number TokenType = "NUMBER" // 10, 30
)

// Token represents a lexical token
type Token struct {
	Type    TokenType
	Literal string
}
