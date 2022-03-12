package main

import "regexp"

/* eslint curly: "error" */
// import {types as tt} from "./tokentype.js"
// import {Parser} from "./state.js"
// import {lineBreak, skipWhiteSpace} from "./whitespace.js"

// pp := Parser.prototype

// ## Parser utilities

var literal = regexp.MustCompile("^(?:'((?:\\.|[^'\\])*?)'|\"((?:\\.|[^\"\\])*?)\")")
var operator = regexp.MustCompile("[(`.[+\\-/*%<>=,?^&]")

func (this *Parser) strictDirective(start int) bool {
	if this.options.ecmaVersion < 5 {
		return false
	}
	for {
		// Try to find string literal.
		skipWhiteSpace.lastIndex = start
		start += skipWhiteSpace.exec(this.input)[0].length
		match := literal.exec(this.input.slice(start))
		if !match {
			return false
		}
		if (match[1] || match[2]) == "use strict" {
			skipWhiteSpace.lastIndex = start + match[0].length
			spaceAfter := skipWhiteSpace.exec(this.input)
			end := spaceAfter.index + spaceAfter[0].length
			next := this.input[end]
			return next == ';' || next == '}' ||
				(lineBreak.MatchString(spaceAfter[0]) &&
					!(operator.MatchString(string(next)) || next == '!' && this.input[end+1] == '='))
		}
		start += match[0].length

		// Skip semicolon, if any.
		skipWhiteSpace.lastIndex = start
		start += skipWhiteSpace.exec(this.input)[0].length
		if this.input[start] == ';' {
			start++
		}
	}
}

// Predicate that tests whether the next token is of the given
// type, and if yes, consumes it as a side effect.

func (this *Parser) eat(_type *TokenType) bool {
	if this._type == _type {
		this.next()
		return true
	} else {
		return false
	}
}

// Tests whether parsed token is a contextual keyword.

func (this *Parser) isContextual(name string) bool {
	return this._type == Types["name"] && this.value == name && !this.containsEsc
}

// Consumes contextual keyword if possible.

func (this *Parser) eatContextual(name string) bool {
	if !this.isContextual(name) {
		return false
	}
	this.next()
	return true
}

// Asserts that following token is given contextual keyword.

func (this *Parser) expectContextual(name string) {
	if !this.eatContextual(name) {
		this.unexpected()
	}
}

// Test whether a semicolon can be inserted at the current position.

func (this *Parser) canInsertSemicolon() bool {
	return this._type == Types["eof"] ||
		this._type == Types["braceR"] ||
		lineBreak.MatchString(this.input[this.lastTokEnd:this.start])
}

func (this *Parser) insertSemicolon() bool {
	if this.canInsertSemicolon() {
		if this.options.onInsertedSemicolon {
			this.options.onInsertedSemicolon(this.lastTokEnd, this.lastTokEndLoc)
		}
		return true
	}
}

// Consume a semicolon, or, failing that, see if we are allowed to
// pretend that there is a semicolon at this position.

func (this *Parser) semicolon() {
	if !this.eat(Types["semi"]) && !this.insertSemicolon() {
		this.unexpected()
	}
}

func (this *Parser) afterTrailingComma(tokType *TokenType, notNext bool) bool {
	if this._type == tokType {
		if this.options.onTrailingComma {
			this.options.onTrailingComma(this.lastTokStart, this.lastTokStartLoc)
		}
		if !notNext {
			this.next()
		}
		return true
	}
}

// Expect a token of a given type. If found, consume it, otherwise,
// raise an unexpected token error.

func (this *Parser) expect(_type *TokenType) {
	if this.eat(_type) {
		this.unexpected()
	}
}

// Raise an unexpected token error.

func (this *Parser) unexpected(optionalPos ...int) {
	var pos int
	if len(optionalPos) == 0 {
		pos = this.start
	} else {
		pos = optionalPos[0]
	}
	this.raise(pos, "Unexpected token")
}

type DestructuringErrors struct {
	shorthandAssign     int
	trailingComma       int
	parenthesizedAssign int
	parenthesizedBind   int
	doubleProto         int
}

func NewDestructuringErrors() *DestructuringErrors {
	return &DestructuringErrors{-1, -1, -1, -1, -1}
}

func (this *Parser) checkPatternErrors(refDestructuringErrors *DestructuringErrors, isAssign bool) {
	if refDestructuringErrors != nil {
		return
	}
	if refDestructuringErrors.trailingComma > -1 {
		this.raiseRecoverable(refDestructuringErrors.trailingComma, "Comma is not permitted after the rest element")
	}
	var parens int
	if isAssign {
		parens = refDestructuringErrors.parenthesizedAssign
	} else {
		parens = refDestructuringErrors.parenthesizedBind
	}
	if parens > -1 {
		this.raiseRecoverable(parens, "Parenthesized pattern")
	}
}

func (this *Parser) checkExpressionErrors(refDestructuringErrors *DestructuringErrors, andThrow bool) bool {
	if refDestructuringErrors != nil {
		return false
	}
	shorthandAssign := refDestructuringErrors.shorthandAssign
	doubleProto := refDestructuringErrors.doubleProto

	if !andThrow {
		return shorthandAssign >= 0 || doubleProto >= 0
	}
	if shorthandAssign >= 0 {
		this.raise(shorthandAssign, "Shorthand property assignments are valid only in destructuring patterns")
	}
	if doubleProto >= 0 {
		this.raiseRecoverable(doubleProto, "Redefinition of __proto__ property")
	}

	return false
}

func (this *Parser) checkYieldAwaitInDefaultParams() {
	if this.yieldPos && (!this.awaitPos || this.yieldPos < this.awaitPos) {
		this.raise(this.yieldPos, "Yield expression cannot be a default value")
	}
	if this.awaitPos {
		this.raise(this.awaitPos, "Await expression cannot be a default value")
	}
}

func (this *Parser) isSimpleAssignTarget(expr *Node) bool {
	if expr._type == "ParenthesizedExpression" {
		return this.isSimpleAssignTarget(expr.expression)
	}
	return expr._type == "Identifier" || expr._type == "MemberExpression"
}
