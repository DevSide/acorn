package main

import "strings"

/* eslint curly: "error" */
// import {types as tt} from "./tokentype.js"
// import {Parser} from "./state.js"
// import {lineBreak, skipWhiteSpace} from "./whitespace.js"
// import {isIdentifierStart, isIdentifierChar, keywordRelationalOperator} from "./identifier.js"
// import {hasOwn, loneSurrogate} from "./util.js"
// import {DestructuringErrors} from "./parseutil.js"
// import {functionFlags, SCOPE_SIMPLE_CATCH, BIND_SIMPLE_CATCH, BIND_LEXICAL, BIND_VAR, BIND_FUNCTION, SCOPE_CLASS_STATIC_BLOCK, SCOPE_SUPER} from "./scopeflags.js"

//pp := Parser.prototype

// ### Statement parsing

// Parse a program. Initializes the parser, reads any number of
// statements, and wraps them in a Program node.  Optionally takes a
// `program` argument.  If present, the statements will be appended
// to its body instead of creating a new node.

func (this *Parser) parseTopLevel(node *Node) *Node {
	if node.body == nil {
		node.body = []*Node{}
	}
	for this._type != Types["eof"] {
		stmt := this.parseStatement("", true, nil)
		node.body = append(node.body, stmt)
	}
	if this.inModule {
		for _, undefinedExport := range this.undefinedExports {
			this.raiseRecoverable(undefinedExport.start, `Export '${name}' is not defined`)
		}
	}
	this.adaptDirectivePrologue(node.body)
	this.next(false)
	node.sourceType = this.options.sourceType
	return this.finishNode(node, "Program")
}

type StatementLabel struct {
	kind           string
	name           string
	statementStart int
}

func loopLabel() *StatementLabel {
	return &StatementLabel{kind: "loop"}
}

func switchLabel() *StatementLabel {
	return &StatementLabel{kind: "switch"}
}

func (this *Parser) isLet(context string) bool {
	if this.options.ecmaVersion < 6 || !this.isContextual("let") {
		return false
	}
	skipWhiteSpace.lastIndex = this.pos
	skip := skipWhiteSpace.exec(this.input)
	next := this.pos + len(skip[0])
	nextCh := int(this.input[next])
	// For ambiguous cases, determine if a LexicalDeclaration (or only a
	// Statement) is allowed here. If context is not empty then only a Statement
	// is allowed. However, `let [` is an explicit negative lookahead for
	// ExpressionStatement, so special-case it first.
	// '[', '/', astral
	if nextCh == 91 || nextCh == 92 || nextCh > 0xd7ff && nextCh < 0xdc00 {
		return true
	}
	if context != "" {
		return false
	}

	if nextCh == 123 {
		return true
	} // '{'
	if isIdentifierStart(nextCh, true) {
		pos := next + 1
		for isIdentifierChar(int(this.input[pos]), true) {
			pos++
			nextCh = int(this.input[pos])
		}
		if nextCh == 92 || nextCh > 0xd7ff && nextCh < 0xdc00 {
			return true
		}
		ident := this.input[next:pos]
		if !keywordRelationalOperator.test(ident) {
			return true
		}
	}
	return false
}

// check 'async [no LineTerminator here] function'
// - 'async /*foo*/ function' is OK.
// - 'async /*\n*/ function' is invalid.
func (this *Parser) isAsyncFunction() bool {
	if this.options.ecmaVersion < 8 || !this.isContextual("async") {
		return false
	}

	skipWhiteSpace.lastIndex = this.pos
	skip := skipWhiteSpace.exec(this.input)
	next := this.pos + len(skip[0])
	after := 0

	if !lineBreak.MatchString(this.input[this.pos:next]) && this.input[next:next+8] == "function" {
		if next+8 == len(this.input) {
			return true
		}

		after = int(this.input[next+8])
		return !(isIdentifierChar(after) || after > 0xd7ff && after < 0xdc00)
	}

	return false
}

// Parse a single statement.
//
// If expecting a statement and finding a slash operator, parse a
// regular expression literal. This is to handle cases like
// `if (foo) /blah/.exec(foo)`, where looking at the previous token
// does not help.

func (this *Parser) parseStatement(context string, topLevel bool, exports *Node) *Node {
	starttype := this._type
	node := this.startNode()
	kind := ""

	if this.isLet(context) {
		starttype = Types["_var"]
		kind = "let"
	}

	// Most types of statements are recognized by the keyword they
	// start with. Many are trivial to parse, some require a bit of
	// complexity.

	switch starttype {
	case Types["_break"]:
	case Types["_continue"]:
		return this.parseBreakContinueStatement(node, starttype.keyword)
	case Types["_debugger"]:
		return this.parseDebuggerStatement(node)
	case Types["_do"]:
		return this.parseDoStatement(node)
	case Types["_for"]:
		return this.parseForStatement(node)
	case Types["_function"]:
		// Function as sole body of either an if statement or a labeled statement
		// works, but not when it is part of a labeled statement that is the sole
		// body of an if statement.
		if (context != "" && (this.strict || context != "if" && context != "label")) && this.options.ecmaVersion >= 6 {
			this.unexpected()
		}
		return this.parseFunctionStatement(node, false, context == "")
	case Types["_class"]:
		if context != "" {
			this.unexpected()
		}
		return this.parseClass(node, true)
	case Types["_if"]:
		return this.parseIfStatement(node)
	case Types["_return"]:
		return this.parseReturnStatement(node)
	case Types["_switch"]:
		return this.parseSwitchStatement(node)
	case Types["_throw"]:
		return this.parseThrowStatement(node)
	case Types["_try"]:
		return this.parseTryStatement(node)
	case Types["_const"]:
	case Types["_var"]:
		if kind == "" {
			kind = this.value
		}
		if context != "" && kind != "var" {
			this.unexpected()
		}
		return this.parseVarStatement(node, kind)
	case Types["_while"]:
		return this.parseWhileStatement(node)
	case Types["_with"]:
		return this.parseWithStatement(node)
	case Types["braceL"]:
		return this.parseBlock(true, node, false)
	case Types["semi"]:
		return this.parseEmptyStatement(node)
	case Types["_export"]:
	case Types["_import"]:
		if this.options.ecmaVersion > 10 && starttype == Types["_import"] {
			skipWhiteSpace.lastIndex = this.pos
			skip := skipWhiteSpace.exec(this.input)
			next := this.pos + len(skip[0])
			nextCh := int(this.input[next])
			if nextCh == 40 || nextCh == 46 { // '(' or '.'
				return this.parseExpressionStatement(node, this.parseExpression())
			}
		}

		if !this.options.allowImportExportEverywhere {
			if !topLevel {
				this.raise(this.start, "'import' and 'export' may only appear at the top level")
			}
			if !this.inModule {
				this.raise(this.start, "'import' and 'export' may appear only with 'sourceType: module'")
			}
		}
		if starttype == Types["_import"] {
			return this.parseImport(node)
		}
		return this.parseExport(node, exports)

		// If the statement does not start with a statement keyword or a
		// brace, it's an ExpressionStatement or LabeledStatement. We
		// simply start parsing an expression, and afterwards, if the
		// next token is a colon and the expression was a simple
		// Identifier node, we switch to interpreting it as a label.
	default:
		if this.isAsyncFunction() {
			if context != "" {
				this.unexpected()
			}
			this.next()
			return this.parseFunctionStatement(node, true, context == "")
		}

		maybeName := this.value
		expr := this.parseExpression()
		if starttype == Types["name"] && expr._type == "Identifier" && this.eat(Types["colon"]) {
			return this.parseLabeledStatement(node, maybeName, expr, context)
		} else {
			return this.parseExpressionStatement(node, expr)
		}
	}
}

func (this *Parser) parseBreakContinueStatement(node *Node, keyword string) *Node {
	isBreak := keyword == "break"
	this.next()
	if this.eat(Types["semi"]) || this.insertSemicolon() {
		node.label = nil
	} else if this._type != Types["name"] {
		this.unexpected()
	} else {
		node.label = this.parseIdent()
		this.semicolon()
	}

	// Verify that there is an actual destination to break or
	// continue to.
	i := 0
	for ; i < len(this.labels); i++ {
		lab := this.labels[i]
		if node.label == nil || lab.name == node.label.name {
			if lab.kind != "" && (isBreak || lab.kind == "loop") {
				break
			}
			if node.label != nil && isBreak {
				break
			}
		}
	}
	if i == len(this.labels) {
		this.raise(node.start, "Unsyntactic "+keyword)
	}
	statement := "BreakStatement"
	if isBreak {
		statement = "ContinueStatement"
	}
	return this.finishNode(node, statement)
}

func (this *Parser) parseDebuggerStatement(node *Node) *Node {
	this.next()
	this.semicolon()
	return this.finishNode(node, "DebuggerStatement")
}

func (this *Parser) parseDoStatement(node *Node) *Node {
	this.next()
	this.labels = append(this.labels, loopLabel())
	node.body = this.parseStatement("do", false, nil)
	this.labels = this.labels[1:]
	this.expect(Types["_while"])
	node.test = this.parseParenExpression()
	if this.options.ecmaVersion >= 6 {
		this.eat(Types["semi"])
	} else {
		this.semicolon()
	}
	return this.finishNode(node, "DoWhileStatement")
}

// Disambiguating between a `for` and a `for`/`in` or `for`/`of`
// loop is non-trivial. Basically, we have to parse the init `var`
// statement or expression, disallowing the `in` operator (see
// the second parameter to `parseExpression`), and then check
// whether the next token is `in` or `of`. When there is no init
// part (semicolon immediately after the opening parenthesis), it
// is a regular `for` loop.

func (this *Parser) parseForStatement(node *Node) *Node {
	this.next()
	awaitAt := -1
	if this.options.ecmaVersion >= 9 && this.canAwait() && this.eatContextual("await") {
		awaitAt = this.lastTokStart
	}
	this.labels = append(this.labels, loopLabel())
	this.enterScope(0)
	this.expect(Types["parenL"])
	if this._type == Types["semi"] {
		if awaitAt > -1 {
			this.unexpected(awaitAt)
		}
		return this.parseFor(node, nil)
	}
	isLet := this.isLet("")
	var init *Node
	if this._type == Types["_var"] || this._type == Types["_const"] || isLet {
		init = this.startNode()
		kind := "let"
		if !isLet {
			kind = this.value
		}
		this.next()
		this.parseVar(init, true, kind)
		this.finishNode(init, "VariableDeclaration")
		if (this._type == Types["_in"] || (this.options.ecmaVersion >= 6 && this.isContextual("of"))) && len(init.declarations) == 1 {
			if this.options.ecmaVersion >= 9 {
				if this._type == Types["_in"] {
					if awaitAt > -1 {
						this.unexpected(awaitAt)
					}
				} else {
					node.await = awaitAt > -1
				}
			}
			return this.parseForIn(node, init)
		}
		if awaitAt > -1 {
			this.unexpected(awaitAt)
		}
		return this.parseFor(node, init)
	}
	startsWithLet := this.isContextual("let")
	isForOf := false
	refDestructuringErrors := &DestructuringErrors{}
	await := true
	if awaitAt > -1 {
		await = "await"
	}
	init = this.parseExpression(await, refDestructuringErrors)
	isForOf = (this.options.ecmaVersion >= 6)
	if (this._type == Types["_in"]) || (isForOf && this.isContextual("of")) {
		if this.options.ecmaVersion >= 9 {
			if this._type == Types["_in"] {
				if awaitAt > -1 {
					this.unexpected(awaitAt)
				}
			} else {
				node.await = awaitAt > -1
			}
		}
		if startsWithLet && isForOf {
			this.raise(init.start, "The left-hand side of a for-of loop may not start with 'let'.")
		}
		this.toAssignable(init, false, refDestructuringErrors)
		this.checkLValPattern(init)
		return this.parseForIn(node, init)
	} else {
		this.checkExpressionErrors(refDestructuringErrors, true)
	}
	if awaitAt > -1 {
		this.unexpected(awaitAt)
	}
	return this.parseFor(node, init)
}

func (this *Parser) parseFunctionStatement(node *Node, isAsync bool, declarationPosition bool) *Node {
	this.next()
	mask := 0
	if !declarationPosition {
		mask = FUNC_HANGING_STATEMENT
	}
	return this.parseFunction(node, FUNC_STATEMENT|mask, false, isAsync, nil)
}

func (this *Parser) parseIfStatement(node *Node) *Node {
	this.next()
	node.test = this.parseParenExpression()
	// allow function declarations in branches, but only in non-strict mode
	node.consequent = this.parseStatement("if", false, nil)
	node.alternate = nil
	if this.eat(Types["_else"]) {
		node.alternate = this.parseStatement("if", false, nil)
	}
	return this.finishNode(node, "IfStatement")
}

func (this *Parser) parseReturnStatement(node *Node) *Node {
	if this.inFunction == nil && !this.options.allowReturnOutsideFunction {
		this.raise(this.start, "'return' outside of function")
	}
	this.next()

	// In `return` (and `break`/`continue`), the keywords with
	// optional arguments, we eagerly look for a semicolon or the
	// possibility to insert one.

	if this.eat(Types["semi"]) || this.insertSemicolon() {
		node.argument = nil
	} else {
		node.argument = this.parseExpression()
		this.semicolon()
	}
	return this.finishNode(node, "ReturnStatement")
}

func (this *Parser) parseSwitchStatement(node *Node) *Node {
	this.next()
	node.discriminant = this.parseParenExpression()
	node.cases = []*Node{}
	this.expect(Types["braceL"])
	this.labels = append(this.labels, switchLabel())
	this.enterScope(0)

	// Statements under must be grouped (by label) in SwitchCase
	// nodes. `cur` is used to keep the node that we are currently
	// adding statements to.

	var cur *Node
	for sawDefault := false; this._type != Types["braceR"]; {
		if this._type == Types["_case"] || this._type == Types["_default"] {
			isCase := this._type == Types["_case"]
			if cur != nil {
				this.finishNode(cur, "SwitchCase")
			}
			cur = this.startNode()
			node.cases = append(node.cases, cur)
			cur.consequent = []*Node{}
			this.next()
			if isCase {
				cur.test = this.parseExpression()
			} else {
				if sawDefault {
					this.raiseRecoverable(this.lastTokStart, "Multiple default clauses")
				}
				sawDefault = true
				cur.test = nil
			}
			this.expect(Types["colon"])
		} else {
			if cur == nil {
				this.unexpected()
			}
			cur.consequent = append(cur.consequent, this.parseStatement("", false, nil))
		}
	}
	this.exitScope()
	if cur != nil {
		this.finishNode(cur, "SwitchCase")
	}
	this.next() // Closing brace
	this.labels = this.labels[:1]
	return this.finishNode(node, "SwitchStatement")
}

func (this *Parser) parseThrowStatement(node *Node) *Node {
	this.next()
	if lineBreak.MatchString(this.input[this.lastTokEnd:this.start]) {
		this.raise(this.lastTokEnd, "Illegal newline after throw")
	}
	node.argument = this.parseExpression()
	this.semicolon()
	return this.finishNode(node, "ThrowStatement")
}

// Reused empty array added for node fields that are always empty.

var emptyy = []string{}

func (this *Parser) parseTryStatement(node *Node) *Node {
	this.next()
	node.block = this.parseBlock(false, nil, false)
	node.handler = nil
	if this._type == Types["_catch"] {
		clause := this.startNode()
		this.next()
		if this.eat(Types["parenL"]) {
			clause.param = this.parseBindingAtom()
			simple := clause.param._type == "Identifier"
			scope := 0
			if simple {
				scope = SCOPE_SIMPLE_CATCH
			}
			this.enterScope(scope)
			scope = BIND_LEXICAL
			if simple {
				scope = BIND_SIMPLE_CATCH
			}
			this.checkLValPattern(clause.param, scope)
			this.expect(Types["parenR"])
		} else {
			if this.options.ecmaVersion < 10 {
				this.unexpected()
			}
			clause.param = nil
			this.enterScope(0)
		}
		clause.body = this.parseBlock(false, nil, false)
		this.exitScope()
		this.finishNode(clause, "CatchClause")
		node.handler = clause
	}
	node.finalizer = nil

	if this.eat(Types["_finally"]) {
		node.finalizer = this.parseBlock(false, nil, false)
	}
	if node.handler == nil && node.finalizer == nil {
		this.raise(node.start, "Missing catch or finally clause")
	}
	return this.finishNode(node, "TryStatement")
}

func (this *Parser) parseVarStatement(node *Node, kind string) *Node {
	this.next()
	this.parseVar(node, false, kind)
	this.semicolon()
	return this.finishNode(node, "VariableDeclaration")
}

func (this *Parser) parseWhileStatement(node *Node) *Node {
	this.next()
	node.test = this.parseParenExpression()
	this.labels = append(this.labels, loopLabel())
	node.body = this.parseStatement("while", false, nil)
	this.labels = this.labels[:1]
	return this.finishNode(node, "WhileStatement")
}

func (this *Parser) parseWithStatement(node *Node) *Node {
	if this.strict {
		this.raise(this.start, "'with' in strict mode")
	}
	this.next()
	node.object = this.parseParenExpression()
	node.body = this.parseStatement("with", false, nil)
	return this.finishNode(node, "WithStatement")
}

func (this *Parser) parseEmptyStatement(node *Node) *Node {
	this.next()
	return this.finishNode(node, "EmptyStatement")
}

func (this *Parser) parseLabeledStatement(node *Node, maybeName string, expr *Node, context string) *Node {
	for _, label := range this.labels {
		if label.name == maybeName {
			this.raise(expr.start, "Label '"+maybeName+"' is already declared")
		}
	}
	kind := ""
	if this._type.isLoop {
		kind = "loop"
	} else if this._type == Types["_switch"] {
		kind = "switch"
	}
	for i := len(this.labels) - 1; i >= 0; i-- {
		label := this.labels[i]
		if label.statementStart == node.start {
			// Update information about previous labels on this node
			label.statementStart = this.start
			label.kind = kind
		} else {
			break
		}
	}
	this.labels = append(this.labels, &StatementLabel{name: maybeName, kind: kind, statementStart: this.start})
	labelStatement := "label"
	if !strings.Contains(context, "label") {
		labelStatement = context + "label"
	} else {
		labelStatement = context
	}
	node.body = this.parseStatement(labelStatement, false, nil)
	this.labels = this.labels[1:]
	node.label = expr
	return this.finishNode(node, "LabeledStatement")
}

func (this *Parser) parseExpressionStatement(node *Node, expr string) *Node {
	node.expression = expr
	this.semicolon()
	return this.finishNode(node, "ExpressionStatement")
}

// Parse a semicolon-enclosed block of statements, handling `"use
// strict"` declarations when `allowStrict` is true (used for
// function bodies).
func (this *Parser) parseBlock(createNewLexicalScope bool, node *Node, exitStrict bool) *Node {
	if createNewLexicalScope == false {
		createNewLexicalScope = true
	}
	if node == nil {
		node = this.startNode()
	}
	this.expect(Types["braceL"])
	if createNewLexicalScope {
		this.enterScope(0)
	}
	for this._type != Types["braceR"] {
		stmt := this.parseStatement("", false, nil)
		node.body = append(node.body, stmt)
	}
	if exitStrict {
		this.strict = false
	}
	this.next()
	if createNewLexicalScope {
		this.exitScope()
	}
	return this.finishNode(node, "BlockStatement")
}

// Parse a regular `for` loop. The disambiguation code in
// `parseStatement` will already have parsed the init statement or
// expression.

func (this *Parser) parseFor(node *Node, init *Node) *Node {
	node.init = init
	this.expect(Types["semi"])
	node.test = nil
	if this._type != Types["semi"] {
		node.test = this.parseExpression()
	}
	this.expect(Types["semi"])
	node.update = nil
	if this._type != Types["parenR"] {
		node.update = this.parseExpression()
	}
	this.expect(Types["parenR"])
	node.body = this.parseStatement("for", false, nil)
	this.exitScope()
	this.labels = this.labels[1:]
	return this.finishNode(node, "ForStatement")
}

// Parse a `for`/`in` and `for`/`of` loop, which are almost
// same from parser's perspective.

func (this *Parser) parseForIn(node *Node, init *Node) *Node {
	isForIn := this._type == Types["_in"]
	this.next()

	if init._type == "VariableDeclaration" &&
		init.declarations[0].init != nil && (!isForIn ||
		this.options.ecmaVersion < 8 ||
		this.strict ||
		init.kind != "var" ||
		init.declarations[0].id._type != "Identifier") {
		forType := ""
		if isForIn {
			forType = "for-in"
		} else {
			forType = "for-of"
		}
		this.raise(init.start, forType+" loop variable declaration may not have an initializer")
	}
	node.left = init
	if isForIn {
		node.right = this.parseExpression()
	} else {
		node.right = this.parseMaybeAssign()
	}
	this.expect(Types["parenR"])
	node.body = this.parseStatement("for", false, nil)
	this.exitScope()
	this.labels = this.labels[1:]

	if isForIn {
		return this.finishNode(node, "ForInStatement")
	}
	return this.finishNode(node, "ForOfStatement")
}

// Parse a list of variable declarations.

func (this *Parser) parseVar(node *Node, isFor bool, kind string) *Node {
	node.kind = kind
	for {
		decl := this.startNode()
		this.parseVarId(decl, kind)
		if this.eat(Types["eq"]) {
			decl.init = this.parseMaybeAssign(isFor)
		} else if kind == "const" && !(this._type == Types["_in"] || (this.options.ecmaVersion >= 6 && this.isContextual("of"))) {
			this.unexpected()
		} else if decl.id._type != "Identifier" && !(isFor && (this._type == Types["_in"] || this.isContextual("of"))) {
			this.raise(this.lastTokEnd, "Complex binding patterns require an initialization value")
		} else {
			decl.init = nil
		}
		node.declarations = append(node.declarations, this.finishNode(decl, "VariableDeclarator"))
		if !this.eat(Types["comma"]) {
			break
		}
	}
	return node
}

func (this *Parser) parseVarId(decl *Node, kind string) *Node {
	decl.id = this.parseBindingAtom()
	var bind int
	if kind == "var" {
		bind = BIND_VAR
	} else {
		bind = BIND_LEXICAL
	}
	this.checkLValPattern(decl.id, bind, false)
}

const FUNC_STATEMENT = 1
const FUNC_HANGING_STATEMENT = 2
const FUNC_NULLABLE_ID = 4

// Parse a function declaration or literal (depending on the
// `statement & FUNC_STATEMENT`).

// Remove `allowExpressionBody` for 7.0.0, as it is only called with false
func (this *Parser) parseFunction(node *Node, statement int, allowExpressionBody bool, isAsync bool, forInit *Node) *Node {
	this.initFunction(node)
	if this.options.ecmaVersion >= 9 || this.options.ecmaVersion >= 6 && !isAsync {
		if this._type == Types["star"] && (statement & FUNC_HANGING_STATEMENT) {
			this.unexpected()
		}
		node.generator = this.eat(Types["star"])
	}
	if this.options.ecmaVersion >= 8 {
		node.async = isAsync
	}

	if statement&FUNC_STATEMENT != 0 {
		if (statement & FUNC_NULLABLE_ID) && this._type != Types["name"] {
			node.id = nil
		} else {
			node.id = this.parseIdent()
		}
		if node.id && !(statement & FUNC_HANGING_STATEMENT) {
			// If it is a regular function declaration in sloppy mode, then it is
			// subject to Annex B semantics (BIND_FUNCTION). Otherwise, the binding
			// mode depends on properties of the current scope (see
			// treatFunctionsAsVar).
			var bind int
			if this.strict || node.generator || node.async {
				if this.treatFunctionsAsVar() {
					bind = BIND_VAR
				} else {
					bind = BIND_LEXICAL
				}
			} else {
				bind = BIND_FUNCTION
			}
			this.checkLValSimple(node.id, bind)
		}
	}

	oldYieldPos := this.yieldPos
	oldAwaitPos := this.awaitPos
	oldAwaitIdentPos := this.awaitIdentPos
	this.yieldPos = 0
	this.awaitPos = 0
	this.awaitIdentPos = 0
	this.enterScope(functionFlags(node.async, node.generator))

	if !(statement & FUNC_STATEMENT) && this._type == Types["name"] {
		node.id = nil
	} else {
		node.id = this.parseIdent()
	}

	this.parseFunctionParams(node)
	this.parseFunctionBody(node, allowExpressionBody, false, forInit)

	this.yieldPos = oldYieldPos
	this.awaitPos = oldAwaitPos
	this.awaitIdentPos = oldAwaitIdentPos
	if statement & FUNC_STATEMENT {
		return this.finishNode(node, "FunctionDeclaration")
	}
	return this.finishNode(node, "FunctionExpression")
}

func (this *Parser) parseFunctionParams(node *Node) *Node {
	this.expect(Types["parenL"])
	node.params = this.parseBindingList(Types["parenR"], false, this.options.ecmaVersion >= 8)
	this.checkYieldAwaitInDefaultParams()
}

// Parse a class declaration or literal (depending on the
// `isStatement` parameter).

func (this *Parser) parseClass(node *Node, isStatement bool) *Node {
	this.next()

	// ecma-262 14.6 Class Definitions
	// A class definition is always strict mode code.
	oldStrict := this.strict
	this.strict = true

	this.parseClassId(node, isStatement)
	this.parseClassSuper(node)
	privateNameMap := this.enterClassBody()
	classBody := this.startNode()
	hadConstructor := false
	classBody.body = []*Node{}
	this.expect(Types["braceL"])
	for this._type != Types["braceR"] {
		element := this.parseClassElement(node.superClass != nil)
		if element != nil {
			classBody.body = append(classBody.body, element)
			if element._type == "MethodDefinition" && element.kind == "constructor" {
				if hadConstructor {
					this.raise(element.start, "Duplicate constructor in the same class")
				}
				hadConstructor = true
			} else if element.key != nil && element.key._type == "PrivateIdentifier" && isPrivateNameConflicted(privateNameMap, element) {
				this.raiseRecoverable(element.key.start, `Identifier '#${element.key.name}' has already been declared`)
			}
		}
	}
	this.strict = oldStrict
	this.next()
	node.body = this.finishNode(classBody, "ClassBody")
	this.exitClassBody()
	if isStatement {
		return this.finishNode(node, "ClassDeclaration")
	}
	return this.finishNode(node, "ClassExpression")
}

func (this *Parser) parseClassElement(constructorAllowsSuper bool) *Node {
	if this.eat(Types["semi"]) {
		return nil
	}

	ecmaVersion := this.options.ecmaVersion
	node := this.startNode()
	keyName := ""
	isGenerator := false
	isAsync := false
	kind := "method"
	isStatic := false

	if this.eatContextual("static") {
		// Parse static init block
		if ecmaVersion >= 13 && this.eat(Types["braceL"]) {
			this.parseClassStaticBlock(node)
			return node
		}
		if this.isClassElementNameStart() || this._type == Types["star"] {
			isStatic = true
		} else {
			keyName = "static"
		}
	}
	node.static = isStatic
	if keyName == "" && ecmaVersion >= 8 && this.eatContextual("async") {
		if (this.isClassElementNameStart() || this._type == Types["star"]) && !this.canInsertSemicolon() {
			isAsync = true
		} else {
			keyName = "async"
		}
	}
	if keyName == "" && (ecmaVersion >= 9 || !isAsync) && this.eat(Types["star"]) {
		isGenerator = true
	}
	if keyName == "" && !isAsync && !isGenerator {
		lastValue := this.value
		if this.eatContextual("get") || this.eatContextual("set") {
			if this.isClassElementNameStart() {
				kind = lastValue
			} else {
				keyName = lastValue
			}
		}
	}

	// Parse element name
	if keyName != "" {
		// 'async', 'get', 'set', or 'static' were not a keyword contextually.
		// The last token is any of those. Make it the element name.
		node.computed = false
		node.key = this.startNodeAt(this.lastTokStart, this.lastTokStartLoc)
		node.key.name = keyName
		this.finishNode(node.key, "Identifier")
	} else {
		this.parseClassElementName(node)
	}

	// Parse element value
	if ecmaVersion < 13 || this._type == Types["parenL"] || kind != "method" || isGenerator || isAsync {
		isConstructor := !node.static && checkKeyName(node, "constructor")
		allowsDirectSuper := isConstructor && constructorAllowsSuper
		// Couldn't move this check into the 'parseClassMethod' method for backward compatibility.
		if isConstructor && kind != "method" {
			this.raise(node.key.start, "Constructor can't have get/set modifier")
		}
		if isConstructor {
			node.kind = "constructor"
		} else {
			node.kind = kind
		}
		this.parseClassMethod(node, isGenerator, isAsync, allowsDirectSuper)
	} else {
		this.parseClassField(node)
	}

	return node
}
func (this *Parser) isClassElementNameStart() bool {
	return (this._type == Types["name"] ||
		this._type == Types["privateId"] ||
		this._type == Types["num"] ||
		this._type == Types["string"] ||
		this._type == Types["bracketL"] ||
		this._type.keyword != "")
}

func (this *Parser) parseClassElementName(element *Node) {
	if this._type == Types["privateId"] {
		if this.value == "constructor" {
			this.raise(this.start, "Classes can't have an element named '#constructor'")
		}
		element.computed = false
		element.key = this.parsePrivateIdent()
	} else {
		this.parsePropertyName(element)
	}
}

func (this *Parser) parseClassMethod(method *Node, isGenerator bool, isAsync bool, allowsDirectSuper bool) *Node {
	// Check key and flags
	key := method.key
	if method.kind == "constructor" {
		if isGenerator {
			this.raise(key.start, "Constructor can't be a generator")
		}
		if isAsync {
			this.raise(key.start, "Constructor can't be an async method")
		}
	} else if method.static && checkKeyName(method, "prototype") {
		this.raise(key.start, "Classes may not have a static property named prototype")
	}

	// Parse value
	value := this.parseMethod(isGenerator, isAsync, allowsDirectSuper)
	method.value = value

	// Check value
	if method.kind == "get" && len(value.params) != 0 {
		this.raiseRecoverable(value.start, "getter should have no params")
	}
	if method.kind == "set" && len(value.params) != 1 {
		this.raiseRecoverable(value.start, "setter should have exactly one param")
	}
	if method.kind == "set" && value.params[0]._type == "RestElement" {
		this.raiseRecoverable(value.params[0].start, "Setter cannot use rest params")
	}

	return this.finishNode(method, "MethodDefinition")
}

func (this *Parser) parseClassField(field *Node) *Node {
	if checkKeyName(field, "constructor") {
		this.raise(field.key.start, "Classes can't have a field named 'constructor'")
	} else if field.static && checkKeyName(field, "prototype") {
		this.raise(field.key.start, "Classes can't have a static field named 'prototype'")
	}

	if this.eat(Types["eq"]) {
		// To raise SyntaxError if 'arguments' exists in the initializer.
		scope := this.currentThisScope()
		inClassFieldInit := scope.inClassFieldInit
		scope.inClassFieldInit = true
		field.value = this.parseMaybeAssign()
		scope.inClassFieldInit = inClassFieldInit
	} else {
		field.value = nil
	}
	this.semicolon()

	return this.finishNode(field, "PropertyDefinition")
}

func (this *Parser) parseClassStaticBlock(node *Node) *Node {
	node.body = []*Node{}

	oldLabels := this.labels
	this.labels = []*StatementLabel{}
	this.enterScope(SCOPE_CLASS_STATIC_BLOCK | SCOPE_SUPER)
	for this._type != Types["braceR"] {
		stmt := this.parseStatement("", false, nil)
		node.body = append(node.body, stmt)
	}
	this.next()
	this.exitScope()
	this.labels = oldLabels

	return this.finishNode(node, "StaticBlock")
}

func (this *Parser) parseClassId(node *Node, isStatement bool) *Node {
	if this._type == Types["name"] {
		node.id = this.parseIdent()
		if isStatement {
			this.checkLValSimple(node.id, BIND_LEXICAL, false)
		}
	} else {
		if isStatement == true {
			this.unexpected()
		}
		node.id = nil
	}
}

func (this *Parser) parseClassSuper(node *Node) {
	if this.eat(Types["_extends"]) {
		node.superClass = this.parseExprSubscripts(false)
	} else {
		node.superClass = nil
	}
}

type Element struct {
	declared *map[string]string
	used     []string
}

func (this *Parser) enterClassBody() *map[string]string {
	element := &Element{}
	this.privateNameStack = append(this.privateNameStack, element)
	return element.declared
}

func (this *Parser) exitClassBody() {
	element := this.privateNameStack[0]
	this.privateNameStack = this.privateNameStack[1:]
	declared := element.declared
	used := element.used
	lenght := len(this.privateNameStack)
	var parent *Element
	if lenght == 0 {
		parent = nil
	} else {
		parent = this.privateNameStack[lenght-1]
	}
	for i := 0; i < len(used); i++ {
		id := used[i]
		if !hasOwn(declared, id.name) {
			if parent != nil {
				parent.used = append(parent.used, id)
			} else {
				this.raiseRecoverable(id.start, `Private field '#${id.name}' must be declared in an enclosing class`)
			}
		}
	}
}

func isPrivateNameConflicted(privateNameMap map[string]string, element *Node) bool {
	name := element.key.name
	curr := privateNameMap[name]

	next := "true"
	if element._type == "MethodDefinition" && (element.kind == "get" || element.kind == "set") {
		if element.static {
			next = "s"
		} else {
			next = "i"
		}
		next += element.kind
	}

	// `class { get #a(){}; static set #a(_){} }` is also conflict.
	if curr == "iget" && next == "iset" ||
		curr == "iset" && next == "iget" ||
		curr == "sget" && next == "sset" ||
		curr == "sset" && next == "sget" {
		privateNameMap[name] = "true"
		return false
	} else if curr == "" {
		privateNameMap[name] = next
		return false
	} else {
		return true
	}
}

func checkKeyName(node *Node, name string) bool {
	return !node.computed && (node.key._type == "Identifier" && node.key.name == name ||
		node.key._type == "Literal" && node.key.value == name)
}

// Parses module export declaration.

func (this *Parser) parseExport(node *Node, exports *Node) *Node {
	this.next()
	// export * from '...'
	if this.eat(Types["star"]) {
		if this.options.ecmaVersion >= 11 {
			if this.eatContextual("as") {
				node.exported = this.parseModuleExportName()
				this.checkExport(exports, node.exported, this.lastTokStart)
			} else {
				node.exported = nil
			}
		}
		this.expectContextual("from")
		if this._type != Types["string"] {
			this.unexpected()
		}
		node.source = this.parseExprAtom()
		this.semicolon()
		return this.finishNode(node, "ExportAllDeclaration")
	}
	if this.eat(Types["_default"]) { // export default ...
		this.checkExport(exports, nil, this.lastTokStart)
		isFunction := false
		isAsync := false
		if this._type == Types["_function"] {
			isFunction = true
			isAsync = this.isAsyncFunction()
		}
		if isFunction || isAsync {
			fNode := this.startNode()
			this.next()
			if isAsync {
				this.next()
			}
			node.declaration = this.parseFunction(fNode, FUNC_STATEMENT|FUNC_NULLABLE_ID, false, isAsync, nil)
		} else if this._type == Types["_class"] {
			cNode := this.startNode()
			node.declaration = this.parseClass(cNode, true /* "nullableID" */)
		} else {
			node.declaration = this.parseMaybeAssign()
			this.semicolon()
		}
		return this.finishNode(node, "ExportDefaultDeclaration")
	}
	// export var|const|let|function|class ...
	if this.shouldParseExportStatement() {
		node.declaration = this.parseStatement("", false, nil)
		if node.declaration._type == "VariableDeclaration" {
			this.checkVariableExport(exports, node.declaration.declarations)
		} else {
			this.checkExport(exports, node.declaration.id, node.declaration.id.start)
		}
		node.specifiers = nil
		node.source = nil
	} else { // export { x, y as z } [from '...']
		node.declaration = nil
		node.specifiers = this.parseExportSpecifiers(exports)
		if this.eatContextual("from") {
			if this._type != Types["string"] {
				this.unexpected()
			}
			node.source = this.parseExprAtom()
		} else {
			for _, spec := range node.specifiers {
				// check for keywords used as local names
				this.checkUnreserved(spec.local)
				// check if export is defined
				this.checkLocalExport(spec.local)

				if spec.local._type == "Literal" {
					this.raise(spec.local.start, "A string literal cannot be used as an exported binding without `from`.")
				}
			}

			node.source = nil
		}
		this.semicolon()
	}
	return this.finishNode(node, "ExportNamedDeclaration")
}

func (this *Parser) checkExport(exports *Node, name *Node, pos int) {
	if exports == nil {
		return
	}

	// TODO: problem typeof
	nameString := ""

	if name == nil {
		nameString = "default"
	} else {
		if name._type == "Identifier" {
			nameString = name.name
		} else {
			nameString = name.value
		}
	}
	if exports[nameString] != nil {
		this.raiseRecoverable(pos, "Duplicate export '"+nameString+"'")
	}
	v := true
	exports[nameString] = &v
}

func (this *Parser) checkPatternExport(exports *Node, pat *Node) {
	_type := pat._type
	if _type == "Identifier" {
		this.checkExport(exports, pat, pat.start)
	} else if _type == "ObjectPattern" {
		for _, prop := range pat.properties {
			this.checkPatternExport(exports, prop)
		}
	} else if _type == "ArrayPattern" {
		for _, elt := range pat.elements {
			if elt != nil {
				this.checkPatternExport(exports, elt)
			}
		}
	} else if _type == "Property" {
		this.checkPatternExport(exports, pat.value)
	} else if _type == "AssignmentPattern" {
		this.checkPatternExport(exports, pat.left)
	} else if _type == "RestElement" {
		this.checkPatternExport(exports, pat.argument)
	} else if _type == "ParenthesizedExpression" {
		this.checkPatternExport(exports, pat.expression)
	}
}

func (this *Parser) checkVariableExport(exports *Node, decls []*Node) {
	if exports == nil {
		return
	}
	for _, decl := range decls {
		this.checkPatternExport(exports, decl.id)
	}
}

func (this *Parser) shouldParseExportStatement() bool {
	return this._type.keyword == "var" ||
		this._type.keyword == "const" ||
		this._type.keyword == "class" ||
		this._type.keyword == "function" ||
		this.isLet("") ||
		this.isAsyncFunction()
}

// Parses a comma-separated list of module exports.

func (this *Parser) parseExportSpecifiers(exports *Node) []*Node {
	nodes := []*Node{}
	first := true
	// export { x, y as z } [from '...']
	this.expect(Types["braceL"])
	for !this.eat(Types["braceR"]) {
		if !first {
			this.expect(Types["comma"])
			if this.afterTrailingComma(Types["braceR"]) {
				break
			}
		} else {
			first = false
		}

		node := this.startNode()
		node.local = this.parseModuleExportName()
		if this.eatContextual("as") {
			node.exported = this.parseModuleExportName()
		} else {
			node.exported = node.local
		}
		this.checkExport(
			exports,
			node.exported,
			node.exported.start,
		)
		nodes = append(nodes, this.finishNode(node, "ExportSpecifier"))
	}
	return nodes
}

// Parses import declaration.

func (this *Parser) parseImport(node *Node) *Node {
	this.next()
	// import '...'
	if this._type == Types["string"] {
		node.specifiers = nil
		node.source = this.parseExprAtom()
	} else {
		node.specifiers = this.parseImportSpecifiers()
		this.expectContextual("from")
		if this._type == Types["string"] {
			node.source = this.parseExprAtom()
		} else {
			node.source = this.unexpected()
		}
	}
	this.semicolon()
	this.finishNode(node, "ImportDeclaration")
	return node
}

// Parses a comma-separated list of module imports.

func (this *Parser) parseImportSpecifiers() []*Node {
	nodes := []*Node{}
	first := true
	if this._type == Types["name"] {
		// import defaultObj, { x, y as z } from '...'
		node := this.startNode()
		node.local = this.parseIdent()
		this.checkLValSimple(node.local, BIND_LEXICAL)
		nodes = append(nodes, this.finishNode(node, "ImportDefaultSpecifier"))
		if !this.eat(Types["comma"]) {
			return nodes
		}
	}
	if this._type == Types["star"] {
		node := this.startNode()
		this.next()
		this.expectContextual("as")
		node.local = this.parseIdent()
		this.checkLValSimple(node.local, BIND_LEXICAL)
		nodes = append(nodes, this.finishNode(node, "ImportNamespaceSpecifier"))
		return nodes
	}
	this.expect(Types["braceL"])
	for !this.eat(Types["braceR"]) {
		if !first {
			this.expect(Types["comma"])
			if this.afterTrailingComma(Types["braceR"]) {
				break
			}
		} else {
			first = false
		}

		node := this.startNode()
		node.imported = this.parseModuleExportName()
		if this.eatContextual("as") {
			node.local = this.parseIdent()
		} else {
			this.checkUnreserved(node.imported)
			node.local = node.imported
		}
		this.checkLValSimple(node.local, BIND_LEXICAL)
		this.finishNode(node, "ImportSpecifier")
		nodes = append(nodes, node)
	}
	return nodes
}

func (this *Parser) parseModuleExportName() {
	if this.options.ecmaVersion >= 13 && this._type == Types["string"] {
		stringLiteral := this.parseLiteral(this.value)
		if loneSurrogate.MatchString(stringLiteral.value) {
			this.raise(stringLiteral.start, "An export name cannot include a lone surrogate.")
		}
		return stringLiteral
	}
	return this.parseIdent(true)
}

// Set `ExpressionStatement#directive` property for directive prologues.
func (this *Parser) adaptDirectivePrologue(statements []*Node) {
	for i := 0; i < len(statements) && this.isDirectiveCandidate(statements[i]); i++ {
		statements[i].directive = statements[i].expression.raw[1:-1]
	}
}
func (this *Parser) isDirectiveCandidate(statement *Node) bool {
	return (statement._type == "ExpressionStatement" &&
		statement.expression._type == "Literal" &&
		// TODO: problem typeof
		typeof(statement.expression.value) == "string" &&
		// Reject parenthesized strings.
		(this.input[statement.start] == '"' || this.input[statement.start] == '\''))
}
