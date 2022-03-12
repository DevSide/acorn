package main

import (
	"fmt"
	"strconv"
)

// import {hasOwn, isArray} from "./util.js"
// import {SourceLocation} from "./locutil.js"

type RawOptions struct {
	ecmaVersion                 string
	sourceType                  string
	onInsertedSemicolon         func(string)
	onTrailingComma             func(string)
	allowReserved               string
	allowReturnOutsideFunction  bool
	allowImportExportEverywhere bool
	allowAwaitOutsideFunction   bool
	allowSuperOutsideMethod     bool
	allowHashBang               bool
	locations                   bool
	onToken                     *Token
	onComment                   func(bool, string, int, int, int, int)
	ranges                      bool
	program                     *Node
	sourceFile                  string
	directSourceFile            string
	preserveParens              bool
}

// A second argument must be given to configure the parser process.
// These options are recognized (only `ecmaVersion` is required):

type Options struct {
	// `ecmaVersion` indicates the ECMAScript version to parse. Must be
	// either 3, 5, 6 (or 2015), 7 (2016), 8 (2017), 9 (2018), 10
	// (2019), 11 (2020), 12 (2021), 13 (2022), or `"latest"` (the
	// latest version the library supports). This influences support
	// for strict mode, the set of reserved words, and support for
	// new syntax features.
	ecmaVersion int
	// `sourceType` indicates the mode the code should be parsed in.
	// Can be either `"script"` or `"module"`. This influences global
	// strict mode and parsing of `import` and `export` declarations.
	sourceType string
	// `onInsertedSemicolon` can be a callback that will be called
	// when a semicolon is automatically inserted. It will be passed
	// the position of the comma as an offset, and if `locations` is
	// enabled, it is given the location as a `{line, column}` object
	// as second argument.
	onInsertedSemicolon func(string)
	// `onTrailingComma` is similar to `onInsertedSemicolon`, but for
	// trailing commas.
	onTrailingComma func(string)
	// By default, reserved words are only enforced if ecmaVersion >= 5.
	// Set `allowReserved` to a boolean value to explicitly turn this on
	// an off. When this option has the value "never", reserved words
	// and keywords can also not be used as property names.
	allowReserved bool
	// When enabled, a return at the top level is not considered an
	// error.
	allowReturnOutsideFunction bool
	// When enabled, import/export statements are not constrained to
	// appearing at the top of the program, and an import.meta expression
	// in a script isn't considered an error.
	allowImportExportEverywhere bool
	// By default, await identifiers are allowed to appear at the top-level scope only if ecmaVersion >= 2022.
	// When enabled, await identifiers are allowed to appear at the top-level scope,
	// but they are still not allowed in non-async functions.
	allowAwaitOutsideFunction bool
	// When enabled, super identifiers are not constrained to
	// appearing in methods and do not raise an error when they appear elsewhere.
	allowSuperOutsideMethod bool
	// When enabled, hashbang directive in the beginning of file
	// is allowed and treated as a line comment.
	allowHashBang bool
	// When `locations` is on, `loc` properties holding objects with
	// `start` and `end` properties in `{line, column}` form (with
	// line being 1-based and column 0-based) will be attached to the
	// nodes.
	locations bool
	// A function can be passed as `onToken` option, which will
	// cause Acorn to call that function with object in the same
	// format as tokens returned from `tokenizer().getToken()`. Note
	// that you are not allowed to call the parser from the
	// callback—that will corrupt its internal state.
	onToken *Token //  func(string)
	// A function can be passed as `onComment` option, which will
	// cause Acorn to call that function with `(block, text, start,
	// end)` parameters whenever a comment is skipped. `block` is a
	// boolean indicating whether this is a block (`/* */`) comment,
	// `text` is the content of the comment, and `start` and `end` are
	// character offsets that denote the start and end of the comment.
	// When the `locations` option is on, two more parameters are
	// passed, the full `{line, column}` locations of the start and
	// end of the comments. Note that you are not allowed to call the
	// parser from the callback—that will corrupt its internal state.
	onComment func(bool, string, int, int, int, int)
	// Nodes have their start and end characters offsets recorded in
	// `start` and `end` properties (directly on the node, rather than
	// the `loc` object, which holds line/column data. To also add a
	// [semi-standardized][range] `range` property holding a `[start,
	// end]` array with the same numbers, set the `ranges` option to
	// `true`.
	//
	// [range]: https://bugzilla.mozilla.org/show_bug.cgi?id=745678
	ranges bool
	// It is possible to parse multiple files into a single AST by
	// passing the tree produced by parsing the first file as
	// `program` option in subsequent parses. This will add the
	// toplevel forms of the parsed file to the `Program` (top) node
	// of an existing parse tree.
	program *Node
	// When `locations` is on, you can pass this to record the source
	// file in every node's `loc` object.
	sourceFile string
	// This value, if given, is stored in every node, whether
	// `locations` is on or off.
	directSourceFile string
	// When enabled, parenthesized expressions are represented by
	// (non-standard) ParenthesizedExpression nodes
	preserveParens bool
}

// Interpret and default an options object

var warnedAboutEcmaVersion = false

func getOptions(opts RawOptions) Options {
	options := Options{}

	if opts.ecmaVersion == "latest" {
		options.ecmaVersion = 1e8
	} else if opts.ecmaVersion == "" {
		if !warnedAboutEcmaVersion {
			warnedAboutEcmaVersion = true
			fmt.Println("WARN: Since Acorn 8.0.0, options.ecmaVersion is required.\nDefaulting to 2020, but this will stop working in the future.")
		}
		options.ecmaVersion = 11
	} else {
		intVar, _ := strconv.Atoi(opts.ecmaVersion)
		options.ecmaVersion = intVar

		if options.ecmaVersion >= 2015 {
			options.ecmaVersion -= 2009
		}
	}

	if opts.sourceType == "" {
		options.sourceType = "script"
	} else {
		options.sourceType = opts.sourceType
	}

	options.onInsertedSemicolon = opts.onInsertedSemicolon
	options.onTrailingComma = opts.onTrailingComma

	if opts.allowReserved == "null" {
		options.allowReserved = options.ecmaVersion < 5
	} else {
		boolValue, _ := strconv.ParseBool(opts.allowReserved)
		options.allowReserved = boolValue
	}

	options.allowReturnOutsideFunction = opts.allowReturnOutsideFunction
	options.allowImportExportEverywhere = opts.allowImportExportEverywhere
	options.allowAwaitOutsideFunction = opts.allowAwaitOutsideFunction
	options.allowSuperOutsideMethod = opts.allowSuperOutsideMethod
	options.allowHashBang = opts.allowHashBang
	options.locations = opts.locations
	options.onToken = opts.onToken
	options.onComment = opts.onComment
	options.ranges = opts.ranges
	options.program = opts.program
	options.sourceFile = opts.sourceFile
	options.directSourceFile = opts.directSourceFile
	options.preserveParens = opts.preserveParens

	return options
}
