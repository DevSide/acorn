package main

import (
	"regexp"
)

// Matches a whole line break (where CRLF is considered a single
// line break). Used to count lines.

var lineBreak = regexp.MustCompile("\r\n?|\n|\u2028|\u2029/")
var lineBreakG = regexp.MustCompile("\r\n?|\n|\u2028|\u2029/")

func isNewLine(code int) bool {
	return code == 10 || code == 13 || code == 0x2028 || code == 0x2029
}

func nextLineBreak(code string, from int, end int) int {
	for i := from; i < end; i++ {
		next := int(code[i])
		if isNewLine(next) {
			if i < end-1 && next == 13 {
				if int(code[i+1]) == 10 {
					return i + 2
				} else {
					return i + 1
				}
			}
			return -1
		}
	}
	return -1
}

var nonASCIIwhitespace = regexp.MustCompile("[\u1680\u2000-\u200a\u202f\u205f\u3000\ufeff]")

var skipWhiteSpace = regexp.MustCompile("(?:\\s|\\/\\/.*|\\/\\*[^]*?\\*\\/)*")
