// cmd/dsl/lexer.go
package dsl

type Lexer struct {
	input        string
	position     int  // current position in input (points to char)
	readPosition int  // reading ahead
	ch           byte // current char
}

func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	switch l.ch {
	case '.':
		tok = Token{Type: Dot, Literal: string(l.ch)}
	case '(':
		tok = Token{Type: Lparen, Literal: string(l.ch)}
	case ')':
		tok = Token{Type: Rparen, Literal: string(l.ch)}
	case ',':
		tok = Token{Type: Comma, Literal: string(l.ch)}
	case '\'':
		l.readPosition++
		str := l.readString()
		tok = Token{Type: String, Literal: str}
	case 0:
		tok = Token{Type: EOF, Literal: ""}
	default:
		if isLetter(l.ch) {
			id := l.readIdentifier()
			tok = Token{Type: Ident, Literal: id}
			return tok
		} else if isDigit(l.ch) {
			num := l.readNumber()
			tok = Token{Type: Number, Literal: num}
			return tok
		} else {
			tok = Token{Type: Illegal, Literal: string(l.ch)}
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	pos := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[pos:l.position]
}

func (l *Lexer) readString() string {
	pos := l.position
	for {
		l.readChar()
		if l.ch == '\'' || l.ch == 0 {
			break
		}
	}
	return l.input[pos:l.position]
}

func (l *Lexer) readNumber() string {
	pos := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[pos:l.position]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
