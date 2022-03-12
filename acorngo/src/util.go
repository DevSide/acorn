package main

import (
	"regexp"
	"strings"
)

func wordsRegexp(words string) *regexp.Regexp {
	result, _ := regexp.Compile("^(?:" + strings.ReplaceAll(words, " ", "|") + ")$")

	return result
}

var loneSurrogate, _ = regexp.Compile("[\\uD800-\\uDFFF]") // u

func includes(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

var MaxInt = int(^uint(0) >> 1)
