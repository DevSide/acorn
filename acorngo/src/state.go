package main

import (
	"regexp"
	"strings"
)

type Parser struct {
	options                  Options
	input                    string
	startPos                 int
	pos                      int
	lineStart                int
	curLine                  int
	sourceFile               string
	keywords                 *regexp.Regexp
	reservedWords            *regexp.Regexp
	reservedWordsStrict      *regexp.Regexp
	reservedWordsStrictBind  *regexp.Regexp
	containsEsc              bool
	_type                    *TokenType
	value                    interface{}
	end                      int
	start                    int
	endLoc                   *Position
	startLoc                 *Position
	lastTokStartLoc          *Position
	lastTokEndLoc            *Position
	lastTokEnd               int
	lastTokStart             int
	context                  []*TokContext
	exprAllowed              bool
	inModule                 bool
	strict                   bool
	potentialArrowAt         int
	potentialArrowInForAwait bool
	yieldPos                 int
	awaitPos                 int
	awaitIdentPos            int
	labels                   []*StatementLabel
	undefinedExports         map[string]*Node
	scopeStack               []*Scope
	regexpState              *regexp.Regexp
	privateNameStack         []*Element
	inTemplateElement        bool
}

func NewParser(originalOptions RawOptions, input string, startPos int) *Parser {
	this := new(Parser)
	options := getOptions(originalOptions)
	this.options = options
	this.sourceFile = options.sourceFile
	key := ""
	if options.ecmaVersion >= 6 {
		key = "6"
	} else if options.sourceType == "module" {
		key = "5module"
	} else {
		key = "5"
	}
	this.keywords = wordsRegexp(keywords[key])
	reserved := ""
	if options.allowReserved != true {
		if options.ecmaVersion >= 6 {
			key = "6"
		} else if options.ecmaVersion == 5 {
			key = "5"
		} else {
			key = "3"
		}
		reserved = reservedWords[key]
		if options.sourceType == "module" {
			reserved += " await"
		}
	}
	this.reservedWords = wordsRegexp(reserved)
	reservedStrict := reservedWords["strict"]
	if reserved != "" {
		reservedStrict = reserved + " " + reservedStrict
	}
	this.reservedWordsStrict = wordsRegexp(reservedStrict)
	this.reservedWordsStrictBind = wordsRegexp(reservedStrict + " " + reservedWords["strictBind"])
	this.input = input

	// Used to signal to callers of `readWord1` whether the word
	// contained any escape sequences. This is needed because words with
	// escape sequences must not be interpreted as keywords.
	this.containsEsc = false

	// Set up token state

	// The current position of the tokenizer in the input.
	if startPos > 0 {
		this.pos = startPos
		this.lineStart = strings.LastIndex(this.input, "\n") + 1
		this.curLine = len(lineBreak.Split(this.input[0:this.lineStart], -1))
	} else {
		this.pos = 0
		this.lineStart = 0
		this.curLine = 1
	}

	// Properties of the current token:
	// Its type
	this._type = Types["eof"]
	// For tokens that include more information than their type, the value
	this.value = ""
	// Its start and end offset
	this.end = this.pos
	this.start = this.end
	// And, if locations are used, the {line, column} object
	// corresponding to those offsets
	this.endLoc = this.curPosition()
	this.startLoc = this.endLoc

	// Position information for the previous token
	this.lastTokStartLoc = nil
	this.lastTokEndLoc = nil
	this.lastTokEnd = this.pos
	this.lastTokStart = this.pos

	// The context stack is used to superficially track syntactic
	// context to predict whether a regular expression is allowed in a
	// given position.
	this.context = this.initialContext()
	this.exprAllowed = true

	// Figure out if it's a module code.
	this.inModule = options.sourceType == "module"
	this.strict = this.inModule || this.strictDirective(this.pos)

	// Used to signify the start of a potential arrow function
	this.potentialArrowAt = -1
	this.potentialArrowInForAwait = false

	// Positions to delayed-check that yield/await does not exist in default parameters.
	this.yieldPos = 0
	this.awaitPos = 0
	this.awaitIdentPos = 0
	// Labels in scope.
	this.labels = []*StatementLabel{}
	// Thus-far undefined exports.
	this.undefinedExports = nil

	// If enabled, skip leading hashbang line.
	if this.pos == 0 && options.allowHashBang && this.input[0:2] == "#!" {
		this.skipLineComment(2)
	}

	// Scope tracking for duplicate variable names (see scope.js)
	this.scopeStack = []*Scope{}
	this.enterScope(SCOPE_TOP)

	// The stack of private names.
	// Each element has two properties: 'declared' and 'used'.
	// When it exited from the outermost class definition, all used private names must be declared.
	this.privateNameStack = []*Element{}

	return this
}

func (this *Parser) parse() *Node {
	node := this.options.program
	if node == nil {
		node = this.startNode()
	}

	this.nextToken()
	return this.parseTopLevel(node)
}

func (this *Parser) inFunction() bool {
	return (this.currentVarScope().flags & SCOPE_FUNCTION) > 0
}

func (this *Parser) inGenerator() bool {
	return (this.currentVarScope().flags&SCOPE_GENERATOR) > 0 && !this.currentVarScope().inClassFieldInit
}

func (this *Parser) inAsync() bool {
	return (this.currentVarScope().flags&SCOPE_ASYNC) > 0 && !this.currentVarScope().inClassFieldInit
}

func (this *Parser) canAwait() bool {
	for i := len(this.scopeStack) - 1; i >= 0; i-- {
		scope := this.scopeStack[i]
		if scope.inClassFieldInit || (scope.flags&SCOPE_CLASS_STATIC_BLOCK != 0) {
			return false
		}
		if scope.flags&SCOPE_FUNCTION != 0 {
			return (scope.flags & SCOPE_ASYNC) > 0
		}
	}
	return (this.inModule && this.options.ecmaVersion >= 13) || this.options.allowAwaitOutsideFunction
}

func (this *Parser) allowSuper() bool {
	scope := this.currentThisScope()
	return (scope.flags&SCOPE_SUPER) > 0 || scope.inClassFieldInit || this.options.allowSuperOutsideMethod
}

func (this *Parser) allowDirectSuper() bool {
	return (this.currentThisScope().flags & SCOPE_DIRECT_SUPER) > 0
}

func (this *Parser) treatFunctionsAsVar() bool {
	return this.treatFunctionsAsVarInScope(this.currentScope())
}

func (this *Parser) allowNewDotTarget() bool {
	scope := this.currentThisScope()
	return (scope.flags&(SCOPE_FUNCTION|SCOPE_CLASS_STATIC_BLOCK)) > 0 || scope.inClassFieldInit
}

func (this *Parser) inClassStaticBlock() bool {
	return (this.currentVarScope().flags & SCOPE_CLASS_STATIC_BLOCK) > 0
}

func extend(plugins) {
	// cls := this
	// for (i := 0; i < plugins.length; i++) { cls = plugins[i](cls) }
	// return cls
}

func parse(input string, options RawOptions) string {
	return NewParser(options, input, -1).parse()
}

func parseExpressionAt(input string, pos int, options RawOptions) {
	parser := NewParser(options, input, pos)
	parser.nextToken()
	return parser.parseExpression()
}

func tokenizer(input string, options RawOptions) *Parser {
	return NewParser(options, input, -1)
}
