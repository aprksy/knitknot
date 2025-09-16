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
	// l.readPosition = 0
	// l.ch = l.input[l.readPosition]
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
	position := l.position

	switch l.ch {
	case '.':
		tok = Token{Type: Dot, Literal: ".", PosX: position}
	case '(':
		tok = Token{Type: LParen, Literal: "(", PosX: position}
	case ')':
		tok = Token{Type: RParen, Literal: ")", PosX: position}
	case ',':
		tok = Token{Type: Comma, Literal: ",", PosX: position}
	case '\'':
		str := l.readString()
		tok = Token{Type: String, Literal: str, PosX: position}
		return tok // ← Return early! Already advanced in readString
	case 0:
		tok = Token{Type: EOF, Literal: ""}
	default:
		if isLetter(l.ch) {
			id := l.readIdentifier()
			tok = Token{Type: Ident, Literal: id, PosX: position}
			return tok // ← Return early! Already advanced
		} else if isDigit(l.ch) {
			num := l.readNumber()
			tok = Token{Type: Number, Literal: num, PosX: position}
			return tok // ← Return early!
		} else {
			tok = Token{Type: Illegal, Literal: string(l.ch), PosX: position}
		}
	}

	// Only advance if we didn't return early
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
	pos := l.position + 1 // start after opening '
	l.readChar()          // consume opening '

	for l.ch != '\'' && l.ch != 0 {
		l.readChar()
	}

	// At this point, l.ch is '\'' or 0
	// Extract string content
	s := l.input[pos:l.position]

	// If we stopped at ', consume it
	if l.ch == '\'' {
		l.readChar() // now points after closing '
	}

	return s
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
