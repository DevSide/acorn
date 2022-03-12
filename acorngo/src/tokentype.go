package main

// ## Token types

// The assignment of fine-grained, information-carrying type objects
// allows the tokenizer to store the information it has about a
// token in a way that is very cheap for the parser to look up.

// All token type variables start with an underscore, to make them
// easy to recognize.

// The `beforeExpr` property is used to disambiguate between regular
// expressions and divisions. It is set on all token types that can
// be followed by an expression (thus, a slash after them would be a
// regular expression).
//
// The `startsExpr` property is used to check if the token ends a
// `yield` expression. It is set on all token types that either can
// directly start an expression (like a quotation mark) or can
// continue an expression (like the body of a string).
//
// `isLoop` marks a keyword as starting a loop, which is important
// to know when parsing a label, in order to allow or disallow
// continue jumps to that label.

type TokenType struct {
	label         string
	keyword       string
	beforeExpr    bool
	startsExpr    bool
	isLoop        bool
	isAssign      bool
	prefix        bool
	postfix       bool
	binop         int
	updateContext string
}

func binop(name string, prec int) *TokenType {
	return &TokenType{label: "num", beforeExpr: true, binop: prec}
}

// Map keyword names to token types.

var Keywords map[string]*TokenType

// Succinct definitions of keyword token types
func kw(name string, token *TokenType) *TokenType {
	token.label = name
	token.keyword = name
	Keywords[name] = token

	return token
}

var Types = map[string]*TokenType{
	"num":       &TokenType{label: "num", startsExpr: true},
	"regexp":    &TokenType{label: "regexp", startsExpr: true},
	"string":    &TokenType{label: "string", startsExpr: true},
	"name":      &TokenType{label: "name", startsExpr: true},
	"privateId": &TokenType{label: "privateId", startsExpr: true},
	"eof":       &TokenType{label: "eof"},

	// Punctuation token types.
	"bracketL":        &TokenType{label: "[", beforeExpr: true, startsExpr: true},
	"bracketR":        &TokenType{label: "]"},
	"braceL":          &TokenType{label: "{", beforeExpr: true, startsExpr: true},
	"braceR":          &TokenType{label: "}"},
	"parenL":          &TokenType{label: "(", beforeExpr: true, startsExpr: true},
	"parenR":          &TokenType{label: ")"},
	"comma":           &TokenType{label: ",", beforeExpr: true},
	"semi":            &TokenType{label: ";", beforeExpr: true},
	"colon":           &TokenType{label: ":", beforeExpr: true},
	"dot":             &TokenType{label: "."},
	"question":        &TokenType{label: "?", beforeExpr: true},
	"questionDot":     &TokenType{label: "?."},
	"arrow":           &TokenType{label: "=>", beforeExpr: true},
	"template":        &TokenType{label: "template"},
	"invalidTemplate": &TokenType{label: "invalidTemplate"},
	"ellipsis":        &TokenType{label: "...", beforeExpr: true},
	"backQuote":       &TokenType{label: "`", startsExpr: true},
	"dollarBraceL":    &TokenType{label: "${", beforeExpr: true, startsExpr: true},

	// Operators. These carry several kinds of properties to help the
	// parser use them properly (the presence of these properties is
	// what categorizes them as operators).
	//
	// `binop`, when present, specifies that this operator is a binary
	// operator, and will refer to its precedence.
	//
	// `prefix` and `postfix` mark the operator as a prefix or postfix
	// unary operator.
	//
	// `isAssign` marks all of `=`, `+=`, `-=` etcetera, which act as
	// binary operators with a very low precedence, that should result
	// in AssignmentExpression nodes.

	"eq":         &TokenType{label: "=", beforeExpr: true, isAssign: true},
	"assign":     &TokenType{label: "_=", beforeExpr: true, isAssign: true},
	"incDec":     &TokenType{label: "++/--", prefix: true, postfix: true, startsExpr: true},
	"prefix":     &TokenType{label: "!/~", beforeExpr: true, prefix: true, startsExpr: true},
	"logicalOR":  binop("||", 1),
	"logicalAND": binop("&&", 2),
	"bitwiseOR":  binop("|", 3),
	"bitwiseXOR": binop("^", 4),
	"bitwiseAND": binop("&", 5),
	"equality":   binop("==/!=/===/!==", 6),
	"relational": binop("</>/<=/>=", 7),
	"bitShift":   binop("<</>>/>>>", 8),
	"plusMin":    &TokenType{label: "+/-", beforeExpr: true, binop: 9, prefix: true, startsExpr: true},
	"modulo":     binop("%", 10),
	"star":       binop("*", 10),
	"slash":      binop("/", 10),
	"starstar":   &TokenType{label: "**", beforeExpr: true},
	"coalesce":   binop("??", 1),

	// Keyword token types.
	"_break":      kw("break", &TokenType{}),
	"_case":       kw("case", &TokenType{beforeExpr: true}),
	"_catch":      kw("catch", &TokenType{}),
	"_continue":   kw("continue", &TokenType{}),
	"_debugger":   kw("debugger", &TokenType{}),
	"_default":    kw("default", &TokenType{beforeExpr: true}),
	"_do":         kw("do", &TokenType{isLoop: true, beforeExpr: true}),
	"_else":       kw("else", &TokenType{beforeExpr: true}),
	"_finally":    kw("finally", &TokenType{}),
	"_for":        kw("for", &TokenType{isLoop: true}),
	"_function":   kw("function", &TokenType{startsExpr: true}),
	"_if":         kw("if", &TokenType{}),
	"_return":     kw("return", &TokenType{beforeExpr: true}),
	"_switch":     kw("switch", &TokenType{}),
	"_throw":      kw("throw", &TokenType{beforeExpr: true}),
	"_try":        kw("try", &TokenType{}),
	"_var":        kw("var", &TokenType{}),
	"_const":      kw("const", &TokenType{}),
	"_while":      kw("while", &TokenType{isLoop: true}),
	"_with":       kw("with", &TokenType{}),
	"_new":        kw("new", &TokenType{beforeExpr: true, startsExpr: true}),
	"_this":       kw("this", &TokenType{startsExpr: true}),
	"_super":      kw("super", &TokenType{startsExpr: true}),
	"_class":      kw("class", &TokenType{startsExpr: true}),
	"_extends":    kw("extends", &TokenType{beforeExpr: true}),
	"_export":     kw("export", &TokenType{}),
	"_import":     kw("import", &TokenType{startsExpr: true}),
	"_null":       kw("null", &TokenType{startsExpr: true}),
	"_true":       kw("true", &TokenType{startsExpr: true}),
	"_false":      kw("false", &TokenType{startsExpr: true}),
	"_in":         kw("in", &TokenType{beforeExpr: true, binop: 7}),
	"_instanceof": kw("instanceof", &TokenType{beforeExpr: true, binop: 7}),
	"_typeof":     kw("typeof", &TokenType{beforeExpr: true, prefix: true, startsExpr: true}),
	"_void":       kw("void", &TokenType{beforeExpr: true, prefix: true, startsExpr: true}),
	"_delete":     kw("delete", &TokenType{beforeExpr: true, prefix: true, startsExpr: true}),
}
