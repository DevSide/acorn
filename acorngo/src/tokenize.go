package main

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

/* eslint curly: "error" */
// import {isIdentifierStart, isIdentifierChar} from "./identifier.js"
// import {types as tt, keywords as keywordTypes} from "./tokentype.js"
// import {Parser} from "./state.js"
// import {SourceLocation} from "./locutil.js"
// import {RegExpValidationState} from "./regexp.js"
// import {lineBreak, nextLineBreak, isNewLine, nonASCIIwhitespace} from "./whitespace.js"

// Object type used to represent tokens. Note that normally, tokens
// simply exist as properties on the parser object. This is only
// used for the onToken callback and the external tokenizer.

type Token struct {
	_type  *TokenType
	value  interface{}
	start  int
	end    int
	loc    *SourceLocation
	_range [2]int
}

func NewToken(p *Parser) *Token {
	var loc *SourceLocation
	if p.options.locations {
		loc = NewSourceLocation(p, p.startLoc, p.endLoc)
	}

	var _range [2]int
	if p.options.ranges {
		_range = [2]int{p.start, p.end}
	}

	return &Token{_type: p._type, value: p.value, start: p.start, end: p.end, loc: loc, _range: _range}
}

// ## Tokenizer

// Move to the next token

func (this *Parser) next(ignoreEscapeSequenceInKeyword ...bool) {
	if !ignoreEscapeSequenceInKeyword[0] && this._type.keyword != "" && this.containsEsc {
		this.raiseRecoverable(this.start, "Escape sequence in keyword "+this._type.keyword)
	}
	if this.options.onToken != nil {
		this.options.onToken(NewToken(this))
	}

	this.lastTokEnd = this.end
	this.lastTokStart = this.start
	this.lastTokEndLoc = this.endLoc
	this.lastTokStartLoc = this.startLoc
	this.nextToken()
}

func (this *Parser) getToken() *Token {
	this.next()
	return NewToken(this)
}

// If we're in an ES6 environment, make parsers iterable
// TODO: Symbol ?
// if (typeof Symbol != "undefined") {
//   pp[Symbol.iterator] = function() {
//     return {
//       next: () => {
//         token := this.getToken()
//         return {
//           done: token.type == tt.eof,
//           value: token
//         }
//       }
//     }
//   }
// }

// Toggle strict mode. Re-reads the next number or string to please
// pedantic tests (`"use strict"; 010;` should fail).

// Read a single token, updating the parser object's token-related
// properties.

func (this *Parser) nextToken() {
	curContext := this.curContext()
	if !curContext || !curContext.preserveSpace {
		this.skipSpace()
	}

	this.start = this.pos
	if this.options.locations {
		this.startLoc = this.curPosition()
	}
	if this.pos >= len(this.input) {
		this.finishToken(Types["eof"], nil)
		return
	}

	if curContext.override {
		curContext.override(this)
	} else {
		this.readToken(this.fullCharCodeAtPos())
	}
}

func (this *Parser) readToken(code int) {
	// Identifier or keyword. '\uXXXX' sequences are allowed in
	// identifiers, so '\' also dispatches to that.
	if isIdentifierStart(code, this.options.ecmaVersion >= 6) || code == 92 /* '\' */ {
		this.readWord()
		return
	}

	this.getTokenFromCode(code)
	return
}

func (this *Parser) fullCharCodeAtPos() int {
	code := int(this.input[this.pos])
	if code <= 0xd7ff || code >= 0xdc00 {
		return code
	}
	next := int(this.input[this.pos+1])

	if next <= 0xdbff || next >= 0xe000 {
		return code
	} else {
		return (code << 10) + next - 0x35fdc00
	}
}

func (this *Parser) skipBlockComment() {
	startLoc := this.options.onComment && this.curPosition()
	start := this.pos
	this.pos += 2
	end := strings.Index(this.input[this.pos:], "*/")
	if end == -1 {
		this.raise(this.pos-2, "Unterminated comment")
	}
	this.pos = end + 2
	if this.options.locations {
		nextBreak := nextLineBreak(this.input, pos, this.pos)
		for pos := start; nextBreak > -1; {
			this.curLine++
			nextBreak = this.lineStart
			pos = nextBreak
			nextBreak = nextLineBreak(this.input, pos, this.pos)
		}
	}
	if this.options.onComment != nil {
		this.options.onComment(true, this.input[start+2:end], start, this.pos,
			startLoc, this.curPosition())
	}
}

func (this *Parser) skipLineComment(startSkip int) {
	start := this.pos
	startLoc := this.options.onComment && this.curPosition()
	this.pos += startSkip
	ch := int(this.input[this.pos])
	for this.pos < len(this.input) && !isNewLine(ch) {
		this.pos++
		ch = int(this.input[this.pos])
	}
	if this.options.onComment != nil {
		this.options.onComment(false, this.input[start+startSkip:this.pos], start, this.pos,
			startLoc, this.curPosition())
	}
}

// Called at the start of the parse and after every token. Skips
// whitespace and comments, and.

func (this *Parser) skipSpace() {
	for this.pos < len(this.input) {
		ch := int(this.input[this.pos])
		switch ch {
		case 32:
		case 160: // ' '
			this.pos++
			break
		case 13:
			if int(this.input[this.pos+1]) == 10 {
				this.pos++
			}
		case 10:
		case 8232:
		case 8233:
			this.pos++
			if this.options.locations {
				this.curLine++
				this.lineStart = this.pos
			}
			break
		case 47: // '/'
			switch int(this.input[this.pos+1]) {
			case 42: // '*'
				this.skipBlockComment()
				break
			case 47:
				this.skipLineComment(2)
				break
			default:
				break loop
			}
			break
		default:
			if ch > 8 && ch < 14 || ch >= 5760 && nonASCIIwhitespace.test(String.fromCharCode(ch)) {
				this.pos++
			} else {
				break loop
			}
		}
	}
}

// Called at the end of every token. Sets `end`, `val`, and
// maintains `context` and `exprAllowed`, and skips the space after
// the token, so that the next one's `start` will point at the
// right position.

func (this *Parser) finishToken(_type *TokenType, val interface{}) {
	this.end = this.pos
	if this.options.locations {
		this.endLoc = this.curPosition()
	}
	prevType := this._type
	this._type = _type
	this.value = val

	this.updateContext(prevType)
}

// ### Token reading

// This is the function that is called to fetch the next token. It
// is somewhat obscure, because it works in character codes rather
// than characters, and because operator parsing has been inlined
// into it.
//
// All in the name of speed.
//
func (this *Parser) readToken_dot() {
	next := int(this.input[this.pos+1])
	if next >= 48 && next <= 57 {
		this.readNumber(true)
		return
	}
	next2 := int(this.input[this.pos+2])
	if this.options.ecmaVersion >= 6 && next == 46 && next2 == 46 { // 46 = dot '.'
		this.pos += 3
		this.finishToken(Types["ellipsis"], nil)
	} else {
		this.pos++
		this.finishToken(Types["dot"], nil)
	}
}

func (this *Parser) readToken_slash() { // '/'
	next := int(this.input[this.pos+1])
	if this.exprAllowed {
		this.pos++
		this.readRegexp()
	} else {
		if next == 61 {
			this.finishOp(Types["assign"], 2)
		} else {
			this.finishOp(Types["slash"], 1)
		}
	}
}

func (this *Parser) readToken_mult_modulo_exp(code int) { // '%*'
	next := int(this.input[this.pos+1])
	size := 1
	var tokentype *TokenType
	if code == 42 {
		tokentype = Types["star"]
	} else {
		tokentype = Types["modulo"]
	}

	// exponentiation operator ** and **=
	if this.options.ecmaVersion >= 7 && code == 42 && next == 42 {
		size++
		tokentype = Types["starstar"]
		next = int(this.input[this.pos+2])
	}

	if next == 61 {
		this.finishOp(Types["assign"], size+1)
	} else {
		this.finishOp(tokentype, size)
	}
}

func (this *Parser) readToken_pipe_amp(code int) { // '|&'
	next := int(this.input[this.pos+1])
	if next == code {
		if this.options.ecmaVersion >= 12 {
			next2 := int(this.input[this.pos+2])
			if next2 == 61 {
				this.finishOp(Types["assign"], 3)
				return
			}
		}
		if code == 124 {
			this.finishOp(Types["logicalOR"], 2)
		} else {
			this.finishOp(Types["logicalAND"], 2)
		}
	} else if next == 61 {
		this.finishOp(Types["assign"], 2)
	} else {
		this.finishOp(Types["bitwiseAND"], 1)
	}
}

func (this *Parser) readToken_caret() { // '^'
	next := int(this.input[this.pos+1])
	if next == 61 {
		this.finishOp(Types["assign"], 2)
	} else {
		this.finishOp(Types["bitwiseXOR"], 1)
	}
}

func (this *Parser) readToken_plus_min(code int) { // '+-'
	next := int(this.input[this.pos+1])
	if next == code {
		if next == 45 && !this.inModule && int(this.input[this.pos+2]) == 62 &&
			(this.lastTokEnd == 0 || lineBreak.test(this.input[this.lastTokEnd:this.pos])) {
			// A `-->` line comment
			this.skipLineComment(3)
			this.skipSpace()
			this.nextToken()
		} else {
			this.finishOp(Types["incDec"], 2)
		}
	} else if next == 61 {
		this.finishOp(Types["assign"], 2)
	} else {
		this.finishOp(Types["plusMin"], 1)
	}
}

func (this *Parser) readToken_lt_gt(code int) { // '<>'
	next := int(this.input[this.pos+1])
	size := 1
	if next == code {
		var size int
		if code == 62 && int(this.input[this.pos+2]) > 0 {
			size = 3
		} else {
			size = 2
		}
		if int(this.input[this.pos+size]) == 61 {
			this.finishOp(Types["assign"], size+1)
		} else {
			this.finishOp(Types["bitShift"], size)
		}
	} else if next == 33 && code == 60 && !this.inModule && int(this.input[this.pos+2]) == 45 &&
		int(this.input[this.pos+3]) == 45 {
		// `<!--`, an XML-style comment that should be interpreted as a line comment
		this.skipLineComment(4)
		this.skipSpace()
		this.nextToken()
	} else if next == 61 {
		size = 2
	} else {
		this.finishOp(Types["relational"], size)
	}
}

func (this *Parser) readToken_eq_excl(code int) { // '=!'
	next := int(this.input[this.pos+1])
	if next == 61 {
		if int(this.input[this.pos+2]) == 61 {
			this.finishOp(Types["equality"], 3)
		} else {
			this.finishOp(Types["equality"], 2)
		}
	} else if code == 61 && next == 62 && this.options.ecmaVersion >= 6 { // '=>'
		this.pos += 2
		this.finishToken(Types["arrow"], nil)
	} else if code == 61 {
		this.finishOp(Types["eq"], 1)
	} else {
		this.finishOp(Types["prefix"], 1)
	}
}

func (this *Parser) readToken_question() { // '?'
	ecmaVersion := this.options.ecmaVersion
	if ecmaVersion >= 11 {
		next := int(this.input[this.pos+1])
		if next == 46 {
			next2 := int(this.input[this.pos+2])
			if next2 < 48 || next2 > 57 {
				this.finishOp(Types["questionDot"], 2)
				return
			}
		}
		if next == 63 {
			if ecmaVersion >= 12 {
				next2 := int(this.input[this.pos+2])
				if next2 == 61 {
					this.finishOp(Types["assign"], 3)
					return
				}
			}
			this.finishOp(Types["coalesce"], 2)
			return
		}
	}
	this.finishOp(Types["question"], 1)
}

func (this *Parser) readToken_numberSign() { // '#'
	ecmaVersion := this.options.ecmaVersion
	code := 35 // '#'
	if ecmaVersion >= 13 {
		this.pos++
		code = this.fullCharCodeAtPos()
		if isIdentifierStart(code, true) || code == 92 /* '\' */ {
			this.finishToken(Types["privateId"], this.readWord1())
			return
		}
	}

	this.raise(this.pos, "Unexpected character '"+codePointToString(code)+"'")
}

func (this *Parser) getTokenFromCode(code int) {
	switch code {
	// The interpretation of a dot depends on whether it is followed
	// by a digit or another two dots.
	case 46: // '.'
		this.readToken_dot()
		return

	// Punctuation tokens.
	case 40:
		this.pos++
		this.finishToken(Types["parenL"], nil)
		return
	case 41:
		this.pos++
		this.finishToken(Types["parenR"], nil)
		return
	case 59:
		this.pos++
		this.finishToken(Types["semi"], nil)
		return
	case 44:
		this.pos++
		this.finishToken(Types["comma"], nil)
		return
	case 91:
		this.pos++
		this.finishToken(Types["bracketL"], nil)
		return
	case 93:
		this.pos++
		this.finishToken(Types["bracketR"], nil)
		return
	case 123:
		this.pos++
		this.finishToken(Types["braceL"], nil)
		return
	case 125:
		this.pos++
		this.finishToken(Types["braceR"], nil)
		return
	case 58:
		this.pos++
		this.finishToken(Types["colon"], nil)
		return

	case 96: // '`'
		if this.options.ecmaVersion < 6 {
			break
		}
		this.pos++
		this.finishToken(Types["backQuote"], nil)
		return

	case 48: // '0'
		next := int(this.input[this.pos+1])
		if next == 120 || next == 88 {
			this.readRadixNumber(16)
			return
		} // '0x', '0X' - hex number
		if this.options.ecmaVersion >= 6 {
			if next == 111 || next == 79 {
				this.readRadixNumber(8)
				return
			} // '0o', '0O' - octal number
			if next == 98 || next == 66 {
				this.readRadixNumber(2)
				return
			} // '0b', '0B' - binary number
		}

	// Anything else beginning with a digit is an integer, octal
	// number, or float.
	case 49:
	case 50:
	case 51:
	case 52:
	case 53:
	case 54:
	case 55:
	case 56:
	case 57: // 1-9
		this.readNumber(false)
		return

	// Quotes produce strings.
	case 34:
	case 39: // '"', "'"
		this.readString(code)
		return

	// Operators are parsed inline in tiny state machines. '=' (61) is
	// often referred to. `finishOp` simply skips the amount of
	// characters it is given as second argument, and returns a token
	// of the type given by its first argument.
	case 47: // '/'
		this.readToken_slash()
		return

	case 37:
	case 42: // '%*'
		this.readToken_mult_modulo_exp(code)
		return

	case 124:
	case 38: // '|&'
		this.readToken_pipe_amp(code)
		return

	case 94: // '^'
		this.readToken_caret()
		return

	case 43:
	case 45: // '+-'
		this.readToken_plus_min(code)
		return

	case 60:
	case 62: // '<>'
		this.readToken_lt_gt(code)
		return

	case 61:
	case 33: // '=!'
		this.readToken_eq_excl(code)
		return

	case 63: // '?'
		this.readToken_question()
		return

	case 126: // '~'
		this.finishOp(Types["prefix"], 1)
		return

	case 35: // '#'
		this.readToken_numberSign()
		return
	}

	this.raise(this.pos, "Unexpected character '"+codePointToString(code)+"'")
}

func (this *Parser) finishOp(_type *TokenType, size int) {
	str := this.input[this.pos : this.pos+size]
	this.pos += size
	this.finishToken(_type, str)
}

func (this *Parser) readRegexp() {
	var escaped bool
	var inClass bool
	start := this.pos
	for {
		if this.pos >= len(this.input) {
			this.raise(start, "Unterminated regular expression")
		}
		ch := this.input[this.pos : this.pos+1]
		if lineBreak.MatchString(ch) {
			this.raise(start, "Unterminated regular expression")
		}
		if !escaped {
			if ch == "[" {
				inClass = true
			} else if ch == "]" && inClass {
				inClass = false
			} else if ch == "/" && !inClass {
				break
			}
			escaped = ch == "\\"
		} else {
			escaped = false
		}
		this.pos++
	}
	pattern := this.input[start:this.pos]
	this.pos++
	flagsStart := this.pos
	flags := this.readWord1()
	if this.containsEsc {
		this.unexpected(flagsStart)
	}

	// Validate pattern
	if this.regexpState == nil {
		this.regexpState = RegExpValidationState(this)
	}
	state := this.regexpState
	state.reset(start, pattern, flags)
	this.validateRegExpFlags(state)
	this.validateRegExpPattern(state)

	// Create Literal#value property value.
	var value *regexp.Regexp
	reg, err := regexp.Compile(pattern, flags)

	if err == nil {
		// ESTree requires null if it failed to instantiate RegExp object.
		// https://github.com/estree/estree/blob/a27003adf4fd7bfad44de9cef372a2eacd527b1c/es5.md#regexpliteral
		value = reg
	}

	this.finishToken(Types["regexp"], map[string]interface{}{pattern: pattern, flags: flags, value: value})
}

// Read an integer in the given radix. Return null if zero digits
// were read, the integer value otherwise. When `len` is given, this
// will return `null` unless the integer has exactly `len` digits.

// TODO: len can be undefined, null or int...
func (this *Parser) readInt(radix int, lenght *int, maybeLegacyOctalNumericLiteral bool) *int {
	// `len` is used for character escape sequences. In that case, disallow separators.
	allowSeparators := this.options.ecmaVersion >= 12 && lenght == nil

	// `maybeLegacyOctalNumericLiteral` is true if it doesn't have prefix (0x,0o,0b)
	// and isn't fraction part nor exponent part. In that case, if the first digit
	// is zero then disallow separators.
	isLegacyOctalNumericLiteral := maybeLegacyOctalNumericLiteral && int(this.input[this.pos]) == 48

	start := this.pos
	total := 0
	lastCode := 0
	var e int
	if lenght == nil {
		e = MaxInt
	} else {
		e = lenght
	}
	for i := 0; i < e; i++ {
		this.pos++
		code := int(this.input[this.pos])
		var val int

		if allowSeparators && code == 95 {
			if isLegacyOctalNumericLiteral {
				this.raiseRecoverable(this.pos, "Numeric separator is not allowed in legacy octal numeric literals")
			}
			if lastCode == 95 {
				this.raiseRecoverable(this.pos, "Numeric separator must be exactly one underscore")
			}
			if i == 0 {
				this.raiseRecoverable(this.pos, "Numeric separator is not allowed at the first of digits")
			}
			lastCode = code
			continue
		}

		if code >= 97 {
			val = code - 97 + 10 // a
		} else if code >= 65 {
			val = code - 65 + 10 // A
		} else if code >= 48 && code <= 57 {
			val = code - 48 // 0-9
		} else {
			val = MaxInt
		}
		if val >= radix {
			break
		}
		lastCode = code
		total = total*radix + val
	}

	if allowSeparators && lastCode == 95 {
		this.raiseRecoverable(this.pos-1, "Numeric separator is not allowed at the last of digits")
	}
	if this.pos == start || lenght != nil && this.pos-start != lenght {
		return nil
	}

	return &total
}

func stringToNumber(str string, isLegacyOctalNumericLiteral bool) int {
	if isLegacyOctalNumericLiteral {
		n, err := strconv.ParseInt(str, 8, 0)
		if err != nil {
			return int(n)
		}
	}

	// `parseFloat(value)` stops parsing at the first numeric separator then returns a wrong value.
	return strconv.ParseFloat(strings.Replace(str, "_", "", -1))
}

func stringToBigInt(str string) *big.Int {
	// `BigInt(value)` throws syntax error if the string contains numeric separators.
	n := new(big.Int)
	n, ok := n.SetString(strings.Replace(str, "_", "", -1), 10)
	if !ok {
		return nil
	}

	return n
}

func (this *Parser) readRadixNumber(radix int) {
	start := this.pos
	this.pos += 2 // 0x
	val := this.readInt(radix, new(int), false)
	if val == nil {
		this.raise(this.start+2, "Expected number in radix "+string(radix))
	}
	if this.options.ecmaVersion >= 11 && int(this.input[this.pos]) == 110 {
		val = stringToBigInt(this.input[start:this.pos])
		this.pos++
	} else if isIdentifierStart(this.fullCharCodeAtPos(), false) {
		this.raise(this.pos, "Identifier directly after number")
	}
	this.finishToken(Types["num"], val)
}

// Read an integer, octal integer, or floating-point number.

func (this *Parser) readNumber(startsWithDot bool) {
	start := this.pos
	if !startsWithDot && this.readInt(10, new(int), true) == nil {
		this.raise(start, "Invalid number")
	}
	octal := this.pos-start >= 2 && int(this.input[start]) == 48
	if octal && this.strict {
		this.raise(start, "Invalid number")
	}
	next := int(this.input[this.pos])
	if !octal && !startsWithDot && this.options.ecmaVersion >= 11 && next == 110 {
		val := stringToBigInt(this.input[start:this.pos])
		this.pos++
		if isIdentifierStart(this.fullCharCodeAtPos(), false) {
			this.raise(this.pos, "Identifier directly after number")
		}
		this.finishToken(Types["num"], val)
		return
	}

	if octal && regexp.MustCompile("[89]").MatchString(this.input[start:this.pos]) {
		octal = false
	}
	if next == 46 && !octal { // '.'
		this.pos++
		this.readInt(10, new(int), false)
		next = int(this.input[this.pos])
	}
	if (next == 69 || next == 101) && !octal { // 'eE'
		this.pos++
		next = int(this.input[this.pos])
		if next == 43 || next == 45 {
			this.pos++
		} // '+-'
		if this.readInt(10, new(int), false) == nil {
			this.raise(start, "Invalid number")
		}
	}
	if isIdentifierStart(this.fullCharCodeAtPos(), false) {
		this.raise(this.pos, "Identifier directly after number")
	}

	val := stringToNumber(this.input[start:this.pos], octal)
	this.finishToken(Types["num"], val)
	return
}

// Read a string value, interpreting backslash-escapes.

func (this *Parser) readCodePoint() int {
	ch := int(this.input[this.pos])
	var code int

	if ch == 123 { // '{'
		if this.options.ecmaVersion < 6 {
			this.unexpected()
		}
		this.pos++
		codePos := this.pos
		code = this.readHexChar(strings.Index(this.input[this.pos:], "}") - this.pos)
		this.pos++
		if code > 0x10FFFF {
			this.invalidStringToken(codePos, "Code point out of bounds")
		}
	} else {
		code = this.readHexChar(4)
	}
	return code
}

func codePointToString(code int) string {
	// UTF-16 Decoding
	if code <= 0xFFFF {
		return string(code)
	}
	code -= 0x10000
	return string((code>>10)+0xD800, (code&1023)+0xDC00)
}

func (this *Parser) readString(quote int) {
	this.pos++
	out := ""
	chunkStart := this.pos
	for {
		if this.pos >= len(this.input) {
			this.raise(this.start, "Unterminated string constant")
		}
		ch := int(this.input[this.pos])
		if ch == quote {
			break
		}
		if ch == 92 { // '\'
			out += this.input[chunkStart:this.pos]
			out += string([]byte{this.readEscapedChar(false)})
			chunkStart = this.pos
		} else if ch == 0x2028 || ch == 0x2029 {
			if this.options.ecmaVersion < 10 {
				this.raise(this.start, "Unterminated string constant")
			}
			this.pos++
			if this.options.locations {
				this.curLine++
				this.lineStart = this.pos
			}
		} else {
			if isNewLine(ch) {
				this.raise(this.start, "Unterminated string constant")
			}
			this.pos++
		}
	}
	this.pos++
	out += this.input[chunkStart:this.pos]
	this.finishToken(Types["string"], out)
}

// Reads template string tokens.

const INVALID_TEMPLATE_ESCAPE_ERROR = "INVALID_TEMPLATE_ESCAPE_ERROR"

func (this *Parser) tryReadTemplateToken() interface{} {
	this.inTemplateElement = true
	err := this.readTmplToken()
	if err == INVALID_TEMPLATE_ESCAPE_ERROR {
		this.readInvalidTemplateToken()
	} else {
		return err
	}

	this.inTemplateElement = false
	return false
}

func (this *Parser) invalidStringToken(position int, message string) string {
	if this.inTemplateElement && this.options.ecmaVersion >= 9 {
		return INVALID_TEMPLATE_ESCAPE_ERROR
	} else {
		this.raise(position, message)
	}
	return ""
}

func (this *Parser) readTmplToken() {
	out := ""
	chunkStart := this.pos
	for {
		if this.pos >= len(this.input) {
			this.raise(this.start, "Unterminated template")
		}
		ch := int(this.input[this.pos])
		if ch == 96 || ch == 36 && int(this.input[this.pos+1]) == 123 { // '`', '${'
			if this.pos == this.start && (this._type == Types["template"] || this._type == Types["invalidTemplate"]) {
				if ch == 36 {
					this.pos += 2
					this.finishToken(Types["dollarBraceL"], nil)
					return
				} else {
					this.pos++
					this.finishToken(Types["backQuote"], nil)
					return
				}
			}
			out += this.input[chunkStart:this.pos]
			this.finishToken(Types["template"], out)
			return
		}
		if ch == 92 { // '\'
			out += this.input[chunkStart:this.pos]
			out += this.readEscapedChar(true)
			chunkStart = this.pos
		} else if isNewLine(ch) {
			out += this.input[chunkStart:this.pos]
			this.pos++
			switch ch {
			case 13:
				if int(this.input[this.pos]) == 10 {
					this.pos++
				}
			case 10:
				out += "\n"
				break
			default:
				out += string(ch)
				break
			}
			if this.options.locations {
				this.curLine++
				this.lineStart = this.pos
			}
			chunkStart = this.pos
		} else {
			this.pos++
		}
	}
}

// Reads a template token to search for the end, without validating any escape sequences
func (this *Parser) readInvalidTemplateToken() {
	for ; this.pos < len(this.input); this.pos++ {
		switch this.input[this.pos] {
		case '\\':
			this.pos++
			break

		case '$':
			if this.input[this.pos+1] != '{' {
				break
			}

		// falls through
		case '`':
			this.finishToken(Types["invalidTemplate"], this.input[this.start:this.pos])
			return

			// no default
		}
	}
	this.raise(this.start, "Unterminated template")
}

// Used to read escaped characters

func (this *Parser) readEscapedChar(inTemplate bool) byte {
	this.pos++
	ch := int(this.input[this.pos])
	this.pos++
	switch ch {
	case 110:
		return '\n' // 'n' -> '\n'
	case 114:
		return '\r' // 'r' -> '\r'
	case 120:
		return string(this.readHexChar(2)) // 'x'
	case 117:
		return codePointToString(this.readCodePoint()) // 'u'
	case 116:
		return '\t' // 't' -> '\t'
	case 98:
		return '\b' // 'b' -> '\b'
	case 118:
		return '\u000b' // 'v' -> '\u000b'
	case 102:
		return '\f' // 'f' -> '\f'
	case 13:
		if int(this.input[this.pos]) == 10 {
			this.pos++
		} // '\r\n'
	case 10: // ' \n'
		if this.options.locations {
			this.lineStart = this.pos
			this.curLine++
		}
		return 0
	case 56:
	case 57:
		if this.strict {
			this.invalidStringToken(
				this.pos-1,
				"Invalid escape sequence",
			)
		}
		if inTemplate {
			codePos := this.pos - 1

			this.invalidStringToken(
				codePos,
				"Invalid escape sequence in template string",
			)

			return 0
		}
	default:
		if ch >= 48 && ch <= 55 {
			octalStr := regexp.MustCompile("^[0-7]+").match(this.input[this.pos-1 : 3])
			octal, _ := strconv.ParseInt(octalStr, 8, 0)

			if octal > 255 {
				octalStr = octalStr[0:-1]
				octal, _ = strconv.ParseInt(octalStr, 8, 0)
			}
			this.pos += len(octalStr) - 1
			ch = int(this.input[this.pos])
			if (octalStr != "0" || ch == 56 || ch == 57) && (this.strict || inTemplate) {
				var message string
				if inTemplate {
					message = "Octal literal in template string"
				} else {
					message = "Octal literal in strict mode"
				}

				this.invalidStringToken(
					this.pos-1-len(octalStr),
					message,
				)
			}
			return byte(octal)
		}
		if isNewLine(ch) {
			// Unicode new line characters after \ get removed from output in both
			// template literals and strings
			return 0
		}
		return byte(ch)
	}
}

// Used to read character escape sequences ('\x', '\u', '\U').

func (this *Parser) readHexChar(len int) *int {
	codePos := this.pos
	n := this.readInt(16, &len, false)
	if n == nil {
		this.invalidStringToken(codePos, "Bad character escape sequence")
	}
	return n
}

// Read an identifier, and return it as a string. Sets `this.containsEsc`
// to whether the word contained a '\u' escape.
//
// Incrementally adds only escaped chars, adding other chunks as-is
// as a micro-optimization.
func (this *Parser) readWord1() string {
	this.containsEsc = false
	word := ""
	first := true
	chunkStart := this.pos
	astral := this.options.ecmaVersion >= 6
	for this.pos < len(this.input) {
		ch := this.fullCharCodeAtPos()
		if isIdentifierChar(ch, astral) {
			if ch <= 0xffff {
				this.pos += 1
			} else {
				this.pos += 2
			}
		} else if ch == 92 { // "\"
			this.containsEsc = true
			word += this.input[chunkStart:this.pos]
			escStart := this.pos
			this.pos++
			if int(this.input[this.pos]) != 117 { // "u"
				this.invalidStringToken(this.pos, "Expecting Unicode escape sequence \\uXXXX")
			}
			this.pos++
			esc := this.readCodePoint()
			var identifier func(int, ...bool) bool
			if first {
				identifier = isIdentifierStart
			} else {
				identifier = isIdentifierChar
			}
			if !identifier(esc, astral) {
				this.invalidStringToken(escStart, "Invalid Unicode escape")
			}
			word += codePointToString(esc)
			chunkStart = this.pos
		} else {
			break
		}
		first = false
	}
	return word + this.input[chunkStart:this.pos]
}

// Read an identifier or keyword token. Will check for reserved
// words when necessary.

func (this *Parser) readWord() {
	word := this.readWord1()
	_type := Types["name"]
	if this.keywords.MatchString(word) {
		_type = keywordTypes[word]
	}
	this.finishToken(_type, word)
}
