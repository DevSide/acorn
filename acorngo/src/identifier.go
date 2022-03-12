package main

import (
	"acorngo/src/generated"
	"regexp"
)

/* eslint curly: "error" */
// These are a run-length and offset encoded representation of the
// >0xffff code points that are a valid part of identifiers. The
// offset starts at 0x10000, and each pair of numbers represents an
// offset to the next range, and then a size of the range.
// import astralIdentifierCodes from "./generated/astralIdentifierCodes.js"
// import astralIdentifierStartCodes from "./generated/astralIdentifierStartCodes.js"
// Big ugly regular expressions that match characters in the
// whitespace, identifier, and identifier-start categories. These
// are only applied when a character is found to actually have a
// code point above 128.
// import nonASCIIidentifierChars from "./generated/nonASCIIidentifierChars.js"
// import nonASCIIidentifierStartChars from "./generated/nonASCIIidentifierStartChars.js"

// Reserved word lists for various dialects of the language

var reservedWords = map[string]string{
	"3":          "abstract boolean byte char class double enum export extends final float goto implements import int interface long native package private protected public short static super synchronized throws transient volatile",
	"5":          "class enum extends super const export import",
	"6":          "enum",
	"strict":     "implements interface let package private protected public static yield",
	"strictBind": "eval arguments",
}

// And the keywords

var ecma5AndLessKeywords = "break case catch continue debugger default do else finally for function if return switch throw try var while with null true false instanceof typeof void delete new in this"

var keywords = map[string]string{
	"5":       ecma5AndLessKeywords,
	"5module": ecma5AndLessKeywords + " export import",
	"6":       ecma5AndLessKeywords + " const class extends export import super",
}

var keywordRelationalOperator, _ = regexp.Compile("^in(stanceof)?$")

// ## Character categories

var nonASCIIidentifierStart, _ = regexp.Compile("[" + generated.NonASCIIidentifierStartChars + "]")
var nonASCIIidentifier, _ = regexp.Compile("[" + generated.NonASCIIidentifierStartChars + generated.NonASCIIidentifierChars + "]")

// This has a complexity linear to the value of the code. The
// assumption is that looking up astral identifier characters is
// rare.
func isInAstralSet(code int, set []int) bool {
	pos := 0x10000
	for i := 0; i < len(set); i += 2 {
		pos += set[i]
		if pos > code {
			return false
		}
		pos += set[i+1]
		if pos >= code {
			return true
		}
	}
	return true
}

// Test whether a given character code starts an identifier.

func isIdentifierStart(code int, astral ...bool) bool {
	if code < 65 {
		return code == 36
	}
	if code < 91 {
		return true
	}
	if code < 97 {
		return code == 95
	}
	if code < 123 {
		return true
	}
	if code <= 0xffff {
		return code >= 0xaa && nonASCIIidentifierStart.MatchString(string(code))
	}
	if astral[0] == false {
		return false
	}
	return isInAstralSet(code, generated.AstralIdentifierStartCodes)
}

// Test whether a given character is part of an identifier.

func isIdentifierChar(code int, astral ...bool) bool {
	if code < 48 {
		return code == 36
	}
	if code < 58 {
		return true
	}
	if code < 65 {
		return false
	}
	if code < 91 {
		return true
	}
	if code < 97 {
		return code == 95
	}
	if code < 123 {
		return true
	}
	if code <= 0xffff {
		return code >= 0xaa && nonASCIIidentifier.MatchString(string(code))
	}
	if astral[0] == false {
		return false
	}
	return isInAstralSet(code, generated.AstralIdentifierStartCodes) || isInAstralSet(code, generated.AstralIdentifierCodes)
}
