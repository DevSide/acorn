package main

// import {nextLineBreak} from "./whitespace.js"

// These are used when `options.locations` is on, for the
// `startLoc` and `endLoc` properties.

type Position struct {
	line   int
	column int
}

// interface Position {
//   line: int
//   column: int
//   offset: int
// }

func (this *Position) offset(n int) *Position {
	return &Position{line: this.line, column: this.column + n}
}

type SourceLocation struct {
	source string
	start  *Position
	end    *Position
}

func NewSourceLocation(p *Parser, start *Position, end ...*Position) *SourceLocation {
	loc := &SourceLocation{start: start}

	if end[0] != nil {
		loc.end = end[0]
	}

	if p.sourceFile != "" {
		loc.source = p.sourceFile
	}

	return loc
}

// The `getLineInfo` function is mostly useful when the
// `locations` option is off (for performance reasons) and you
// want to find the line/column position for a given character
// offset. `input` should be the code string that the offset refers
// into.

func getLineInfo(input string, offset int) *Position {
	nextBreak := 0
	line := 1
	cur := 0
	for nextBreak >= 0 {
		nextBreak = nextLineBreak(input, cur, offset)
		if nextBreak < 0 {
			return &Position{line: line, column: offset - cur}
		}
		line++
		cur = nextBreak
	}

	return nil
}
