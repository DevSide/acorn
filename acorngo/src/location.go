package main

import (
	"log"
	"os"
)

// import {Parser} from "./state.js"
// import {Position, getLineInfo} from "./locutil.js"

// const pp = Parser.prototype

// This function is used to raise exceptions on parse errors. It
// takes an offset integer (into the current `input`) to indicate
// the location of the error, attaches the position to the end
// of the error message, and then raises a `SyntaxError` with that
// message.

func (this *Parser) raise(pos int, message string) {
	loc := getLineInfo(this.input, pos)
	message += " (" + loc.line + ":" + loc.column + ")"
	err := new SyntaxError(message)
	err.pos = pos; err.loc = loc; err.raisedAt = this.pos
	log.Fatal(message)
	os.Exit(1)
}

func (this *Parser) raiseRecoverable(pos int, message string) {
	this.raise(pos, message)
}

func (this *Parser) curPosition() *Position {
	if this.options.locations {
		return &Position{line: this.curLine, column: this.pos - this.lineStart}
	}

	return nil
}
