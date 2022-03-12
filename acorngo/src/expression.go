package main
/* eslint curly: "error" */
// A recursive descent parser operates by defining functions for all
// syntactic elements, and recursively calling those, each function
// advancing the input stream and returning an AST node. Precedence
// of constructs (for example, the fact that `!x[1]` means `!(x[1])`
// instead of `(!x)[1]` is handled by the fact that the parser
// function that parses unary prefix operators is called first, and
// in turn calls the function that parses `[]` subscripts — that
// way, it'll receive the node for `x[1]` already parsed, and wraps
// *that* in the unary operator node.
//
// Acorn uses an [operator precedence parser][opp] to handle binary
// operator precedence, because it is much more compact than using
// the technique outlined above, which uses different, nesting
// functions to specify precedence, for all of the ten binary
// precedence levels that JavaScript defines.
//
// [opp]: http://en.wikipedia.org/wiki/Operator-precedence_parser

// import ._types as tt} from "./toke.type.js"
// import ._types as tokenCt.types} from "./tokencontext.js"
// import {Parser} from "./state.js"
// import {DestructuringErrors} from "./parseutil.js"
// import {lineBreak} from "./whitespace.js"
// import {functionFlags, SCOPE_ARROW, SCOPE_SUPER, SCOPE_DIRECT_SUPER, BIND_OUTSIDE, BIND_VAR} from "./scopeflags.js"

// Check if property name clashes with already added.
// Object/class getters and setters are not allowed to clash —
// either with each other or with an init property — and in
// strict mode, init properties are also not allowed to be repeated.

func (this *Parser) checkPropClash (prop, propHash, refDestructuringErrors *DestructuringErrors) {
   if this.options.ecmaVersion >= 9 && prop._type == "SpreadElement" {
    return
  }
  if (this.options.ecmaVersion >= 6 && (prop.computed || prop.method || prop.shorthand)) {
    return
  }
  key := prop.key
  name := ""
  switch (key._type) {
  case "Identifier": name = key.name; break
  case "Literal": name = String(key.value); break
  default: return
  }
  let {kind} = prop
   if this.options.ecmaVersion >= 6 {
     if name == "__proto__" && kind == "init" {
       if propHash.proto {
         if refDestructuringErrors {
           if refDestructuringErrors.doubleProto < 0 {
            refDestructuringErrors.doubleProto = key.start
          }
        } else {
          this.raiseRecoverable(key.start, "Redefinition of __proto__ property")
        }
      }
      propHash.proto = true
    }
    return
  }
  name = "$" + name
  other := propHash[name]
   if other {
    redefinition := false
     if kind == "init" {
      redefinition = this.strict && other.init || other.get || other.set
    } else {
      redefinition = other.init || other[kind]
    }
    if (redefinition) {
      this.raiseRecoverable(key.start, "Redefinition of property")
    }
  } else {
    propHash[name] = &What3{
      init: false,
      get: false,
      set: false,
    }
    other = propHash[name]
  }
  other[kind] = true
}

// ### Expression parsing

// These nest, from the most general expression._type at the top to
// 'atomic', nondivisible expression._types at the bottom. Most of
// the functions will simply let the function(s) below them parse,
// and, *if* the syntactic construct they handle is present, wrap
// the AST node that the inner parser gave them in another node.

// Parse a full expression. The optional arguments are used to
// forbid the `in` operator (in for loops initalization expressions)
// and provide reference for storing '=' operator inside shorthand
// property assignment in contexts where both object expression
// and object pattern might appear (so it's possible to raise
// delayed syntax error at correct position).

func (this *Parser) parseExpression (forInit, refDestructuringErrors) {
  startPos := this.start
  startLoc := this.startLoc
  expr := this.parseMaybeAssign(forInit, refDestructuringErrors)
  if this._type == Types["comma"] {
    node := this.startNodeAt(startPos, startLoc)
    node.expressions = &What4{expr}
    for (this.eat(Types["comma"])) {
      node.expressions.push(this.parseMaybeAssign(forInit, refDestructuringErrors))
    }
    return this.finishNode(node, "SequenceExpression")
  }
  return expr
}

// Parse an assignment expression. This includes applications of
// operators like `+=`.

func (this *Parser) parseMaybeAssign (forInit, refDestructuringErrors, afterLeftParse) {
  if (this.isContextual("yield")) {
    if (this.inGenerator) {
      return this.parseYield(forInit)
    }
    // The tokenizer will assume an expression is allowed after
    // `yield`, but this isn't that kind of yield
    this.exprAllowed = false
  }

  ownDestructuringErrors := false
  oldParenAssign := -1
  oldTrailingComma := -1
  oldDoubleProto := -1
   if refDestructuringErrors {
    oldParenAssign = refDestructuringErrors.parenthesizedAssign
    oldTrailingComma = refDestructuringErrors.trailingComma
    oldDoubleProto = refDestructuringErrors.doubleProto
    refDestructuringErrors.parenthesizedAssign = -1
    refDestructuringErrors.trailingComma = -1
  } else {
    refDestructuringErrors = &DestructuringErrors{}
    ownDestructuringErrors = true
  }

  startPos := this.start
  startLoc := this.startLoc
   if this._type == Types["parenL"] || this._type == Types["name"] {
    this.potentialArrowAt = this.start
    this.potentialArrowInForAwait = forInit == "await"
  }
  left := this.parseMaybeConditional(forInit, refDestructuringErrors)
  if (afterLeftParse) {
    left = afterLeftParse.call(this, left, startPos, startLoc)
  }
   if this._type.isAssign {
    node := this.startNodeAt(startPos, startLoc)
    node.operator = this.value
    if (this._type == Types["eq"]) {
      left = this.toAssignable(left, false, refDestructuringErrors)

    }
     if !ownDestructuringErrors {
      refDestructuringErrors.parenthesizedAssign = -1
      refDestructuringErrors.trailingComma = -1
      refDestructuringErrors.doubleProto = -1
    }
    if (refDestructuringErrors.shorthandAssign >= left.start) {
      refDestructuringErrors.shorthandAssign = -1 // reset because shorthand default was used correctly

    }
    if (this._type == Types["eq"]) {
      this.checkLValPattern(left)
    } else {
      this.checkLValSimple(left)
    }
    node.left = left
    this.next()
    node.right = this.parseMaybeAssign(forInit)
    if oldDoubleProto > -1 {
      refDestructuringErrors.doubleProto = oldDoubleProto
    }
    return this.finishNode(node, "AssignmentExpression")
  } else {
    if (ownDestructuringErrors) {
      this.checkExpressionErrors(refDestructuringErrors, true)
    }
  }
  if oldParenAssign > -1 {
    refDestructuringErrors.parenthesizedAssign = oldParenAssign
  }
  if oldTrailingComma > -1 {
    refDestructuringErrors.trailingComma = oldTrailingComma
  }
  return left
}

// Parse a ternary conditional (`?:`) operator.

func (this *Parser) parseMaybeConditional (forInit, refDestructuringErrors) {
  startPos := this.start
  startLoc := this.startLoc
  expr := this.parseExprOps(forInit, refDestructuringErrors)
  if this.checkExpressionErrors(refDestructuringErrors) {
    return expr
  }
  if (this.eat(Types["question"])) {
    node := this.startNodeAt(startPos, startLoc)
    node.test = expr
    node.consequent = this.parseMaybeAssign()
    this.expect(Types["colon"])
    node.alternate = this.parseMaybeAssign(forInit)
    return this.finishNode(node, "ConditionalExpression")
  }
  return expr
}

// Start the precedence parser.

func (this *Parser) parseExprOps (forInit, refDestructuringErrors) {
  startPos := this.start
  startLoc := this.startLoc
  expr := this.parseMaybeUnary(refDestructuringErrors, false, false, forInit)
  if (this.checkExpressionErrors(refDestructuringErrors)) {
    return expr
  }
  if (expr.start == startPos && expr._type == "ArrowFunctionExpression") {
    return expr
  }
  return this.parseExprOp(expr, startPos, startLoc, -1, forInit)
}

// Parse binary operators with the operator precedence parsing
// algorithm. `left` is the left-hand side of the operator.
// `minPrec` provides context that allows the function to stop and
// defer further parser to one of its callers when it encounters an
// operator that has a lower precedence than the set it is parsing.

func (this *Parser) parseExprOp (left, leftStartPos, leftStartLoc, minPrec, forInit) {
  prec := this._type.binop
  if (prec != null && (!forInit || this._type != Types["_in"])) {
     if prec > minPrec {
      logical := this._type == Types["logicalOR"] || this._type == Types["logicalAND"]
      coalesce := this._type == Types["coalesce"]
       if coalesce {
        // Handle the precedence of `Types["coalesce"]` as equal to the range of logical expressions.
        // In other words, `node.right` shouldn't contain logical expressions in order to check the mixed error.
        prec = Types["logicalAND"].binop
      }
      op := this.value
      this.next()
      startPos := this.start
      startLoc := this.startLoc
      right := this.parseExprOp(this.parseMaybeUnary(null, false, false, forInit), startPos, startLoc, prec, forInit)
      node := this.buildBinary(leftStartPos, leftStartLoc, left, right, op, logical || coalesce)
      if ((logical && this._type == Types["coalesce"]) || (coalesce && (this._type == Types["logicalOR"] || this._type == Types["logicalAND"]))) {
        this.raiseRecoverable(this.start, "Logical expressions and coalesce expressions cannot be mixed. Wrap either by parentheses")
      }
      return this.parseExprOp(node, leftStartPos, leftStartLoc, minPrec, forInit)
    }
  }
  return left
}

func (this *Parser) buildBinary (startPos, startLoc, left, right, op, logical) {
  if (right._type == "PrivateIdentifier") {
    this.raise(right.start, "Private identifier can only be left side of binary expression")
  }
  node := this.startNodeAt(startPos, startLoc)
  node.left = left
  node.operator = op
  node.right = right

  if (logical) {
    return this.finishNode(node, "LogicalExpression")
  }
  return this.finishNode(node, "BinaryExpression")
}

// Parse unary operators, both prefix and postfix.

func (this *Parser) parseMaybeUnary (refDestructuringErrors, sawUnary, incDec, forInit) {
  startPos := this.start
  startLoc := this.startLoc
  var expr *Node
  if (this.isContextual("await") && this.canAwait) {
    expr = this.parseAwait(forInit)
    sawUnary = true
  } else  if this._type.prefix {
    node := this.startNode()
    update := this._type == Types["incDec"]
    node.operator = this.value
    node.prefix = true
    this.next()
    node.argument = this.parseMaybeUnary(null, true, update, forInit)
    this.checkExpressionErrors(refDestructuringErrors, true)
    if (update) {
      this.checkLValSimple(node.argument)
    } else if (this.strict && node.operator == "delete" && node.argument._type == "Identifier") {
              this.raiseRecoverable(node.start, "Deleting local variable in strict mode")

             }else if (node.operator == "delete" && isPrivateFieldAccess(node.argument)) {
              this.raiseRecoverable(node.start, "Private fields can not be deleted")

             } else {
              sawUnary = true
             }
             if (update) {
              expr = this.finishNode(node, "UpdateExpression")
             } else {
              expr = this.finishNode(node, "UnaryExpression")
             }
  } else  if !sawUnary && this._type == Types["privateId"] {
    if (forInit || len(this.privateNameStack) == 0) {
      this.unexpected()
    }
    expr = this.parsePrivateIdent()
    // only could be private fields in 'in', such as #x in obj
    if (this._type != Types["_in"]) {
      this.unexpected()
    }
  } else {
    expr = this.parseExprSubscripts(refDestructuringErrors, forInit)
    if (this.checkExpressionErrors(refDestructuringErrors)) {
      return expr
    }
    for (this._type.postfix && !this.canInsertSemicolon()) {
      node := this.startNodeAt(startPos, startLoc)
      node.operator = this.value
      node.prefix = false
      node.argument = expr
      this.checkLValSimple(expr)
      this.next()
      expr = this.finishNode(node, "UpdateExpression")
    }
  }

  if (!incDec && this.eat(Types["starstar"])) {
    if (sawUnary) {
      this.unexpected(this.lastTokStart)
    } else {
      return this.buildBinary(startPos, startLoc, expr, this.parseMaybeUnary(null, false, false, forInit), "**", false)
    }
  } else {
    return expr
  }
}

func isPrivateFieldAccess(node) {
  return (
    node._type == "MemberExpression" && node.property._type == "PrivateIdentifier" ||
    node._type == "ChainExpression" && isPrivateFieldAccess(node.expression))
}

// Parse call, dot, and `[]`-subscript expressions.

func (this *Parser) parseExprSubscripts (refDestructuringErrors, forInit) {
  startPos := this.start
  startLoc := this.startLoc
  expr := this.parseExprAtom(refDestructuringErrors, forInit)
  if (expr._type == "ArrowFunctionExpression" && this.input[this.lastTokStart: this.lastTokEnd] != ")")  {
    return expr
  }
  result := this.parseSubscripts(expr, startPos, startLoc, false, forInit)
   if refDestructuringErrors && result._type == "MemberExpression" {
    if (refDestructuringErrors.parenthesizedAssign >= result.start) { 
      refDestructuringErrors.parenthesizedAssign = -1
    }
    if (refDestructuringErrors.parenthesizedBind >= result.start) {
      refDestructuringErrors.parenthesizedBind = -1
    }
    if (refDestructuringErrors.trailingComma >= result.start) {
      refDestructuringErrors.trailingComma = -1
    }
  }
  return result
}

func (this *Parser) parseSubscripts (base, startPos, startLoc, noCalls, forInit) {
  maybeAsyncArrow := this.options.ecmaVersion >= 8 && base._type == "Identifier" && base.name == "async" &&
      this.lastTokEnd == base.end && !this.canInsertSemicolon() && base.end - base.start == 5 &&
      this.potentialArrowAt == base.start
  optionalChained := false

  for (true) {
    element := this.parseSubscript(base, startPos, startLoc, noCalls, maybeAsyncArrow, optionalChained, forInit)

    if (element.optional) {
      optionalChained = true
    }
     if element == base || element._type == "ArrowFunctionExpression" {
       if optionalChained {
        chainNode := this.startNodeAt(startPos, startLoc)
        chainNode.expression = element
        element = this.finishNode(chainNode, "ChainExpression")
      }
      return element
    }

    base = element
  }
}

func (this *Parser) parseSubscript (base, startPos, startLoc, noCalls, maybeAsyncArrow, optionalChained, forInit) {
  optionalSupported := this.options.ecmaVersion >= 11
  optional := optionalSupported && this.eat(Types["questionDot"])
  if (noCalls && optional) {
    this.raise(this.lastTokStart, "Optional chaining cannot appear in the callee of new expressions")
  }

  computed := this.eat(Types["bracketL"])
  if (computed || (optional && this._type != Types["parenL"] && this._type != Types["backQuote"]) || this.eat(Types["dot"])) {
    node := this.startNodeAt(startPos, startLoc)
    node.object = base
     if computed {
      node.property = this.parseExpression()
      this.expect(Types["bracketR"])
    } else  if this._type == Types["privateId"] && base._type != "Super" {
      node.property = this.parsePrivateIdent()
    } else {
      node.property = this.parseIdent(this.options.allowReserved != "never")
    }
    node.computed = !!computed
     if optionalSupported {
      node.optional = optional
    }
    base = this.finishNode(node, "MemberExpression")
  } else if (!noCalls && this.eat(Types["parenL"])) {
    refDestructuringErrors := &DestructuringErrors
    oldYieldPos := this.yieldPos
    oldAwaitPos := this.awaitPos
    oldAwaitIdentPos := this.awaitIdentPos
    this.yieldPos = 0
    this.awaitPos = 0
    this.awaitIdentPos = 0
    exprList := this.parseExprList(Types["parenR"], this.options.ecmaVersion >= 8, false, refDestructuringErrors)
    if (maybeAsyncArrow && !optional && !this.canInsertSemicolon() && this.eat(Types["arrow"])) {
      this.checkPatternErrors(refDestructuringErrors, false)
      this.checkYieldAwaitInDefaultParams()
      if (this.awaitIdentPos > 0) {
        this.raise(this.awaitIdentPos, "Cannot use 'await' as identifier inside an async function")
      }
      this.yieldPos = oldYieldPos
      this.awaitPos = oldAwaitPos
      this.awaitIdentPos = oldAwaitIdentPos
      return this.parseArrowExpression(this.startNodeAt(startPos, startLoc), exprList, true, forInit)
    }
    this.checkExpressionErrors(refDestructuringErrors, true)
    this.yieldPos = oldYieldPos || this.yieldPos
    this.awaitPos = oldAwaitPos || this.awaitPos
    this.awaitIdentPos = oldAwaitIdentPos || this.awaitIdentPos
    node := this.startNodeAt(startPos, startLoc)
    node.callee = base
    node.arguments = exprList
     if optionalSupported {
      node.optional = optional
    }
    base = this.finishNode(node, "CallExpression")
  } else if this._type == Types["backQuote"] {
     if optional || optionalChained {
      this.raise(this.start, "Optional chaining cannot appear in the tag of tagged template expressions")
    }
    node := this.startNodeAt(startPos, startLoc)
    node.tag = base
    node.quasi = this.parseTemplate(&Unknown{isTagged: true})
    base = this.finishNode(node, "TaggedTemplateExpression")
  }
  return base
}

// Parse an atomic expression — either a single token that is an
// expression, an expression started by a keyword like `function` or
// `new`, or an expression wrapped in punctuation like `()`, `[]`,
// or `{}`.

func (this *Parser) parseExprAtom (refDestructuringErrors, forInit) {
  // If a division operator appears in an expression position, the
  // tokenizer got confused, and we force it to read a regexp instead.
  if (this._type == Types["slash"]) {
    this.readRegexp()
  }

  var node *Node
  canBeArrow := this.potentialArrowAt == this.start
  switch (this._type) {
  case Types["_super"]:
    if (!this.allowSuper) {
      this.raise(this.start, "'super' keyword outside a method")
    }
    node = this.startNode()
    this.next()
    if (this._type == Types["parenL"] && !this.allowDirectSuper) {
      this.raise(node.start, "super() call outside constructor of a subclass")
    }
    // The `super` keyword can appear at below:
    // SuperProperty:
    //     super [ Expression ]
    //     super . IdentifierName
    // SuperCall:
    //     super ( Arguments )
    if (this._type != Types["dot"] && this._type != Types["bracketL"] && this._type != Types["parenL"]) {
      this.unexpected()
    }
    return this.finishNode(node, "Super")

  case Types["_this"]:
    node = this.startNode()
    this.next()
    return this.finishNode(node, "ThisExpression")

  case Types["name"]:
    startPos := this.start
    startLoc := this.startLoc
    containsEsc := this.containsEsc
    id := this.parseIdent(false)
    if (this.options.ecmaVersion >= 8 && !containsEsc && id.name == "async" && !this.canInsertSemicolon() && this.eat(Types["_function"])) {
      this.overrideContext(tokenCt._types.f_expr)
      return this.parseFunction(this.startNodeAt(startPos, startLoc), 0, false, true, forInit)
    }
    if (canBeArrow && !this.canInsertSemicolon()) {
      if (this.eat(Types["arrow"])) {
        return this.parseArrowExpression(this.startNodeAt(startPos, startLoc), &[]string{id}, false, forInit)
      }
      if (this.options.ecmaVersion >= 8 && id.name == "async" && this._type == Types["name"] && !containsEsc &&
          (!this.potentialArrowInForAwait || this.value != "of" || this.containsEsc)) {
        id = this.parseIdent(false)
        if (this.canInsertSemicolon() || !this.eat(Types["arrow"])) {
          this.unexpected()
        }
        return this.parseArrowExpression(this.startNodeAt(startPos, startLoc), &[]string{id}, true, forInit)
      }
    }
    return id

  case Types["regexp"]:
    value := this.value
    node = this.parseLiteral(value.value)
    node.regex = &map[string]string{pattern: value.pattern, flags: value.flags}
    return node

  case Types["num"]: case Types["string"]:
    return this.parseLiteral(this.value)

  case Types["_null"]: case Types["_true"]: case Types["_false"]:
    node = this.startNode()
    node._type = nil
    if (this._type != Types["_null"]) {
      node._type = this._type == Types["_true"]
    }
    node.raw = this._type.keyword
    this.next()
    return this.finishNode(node, "Literal")

  case Types["parenL"]:
    start := this.start
    expr := this.parseParenAndDistinguishExpression(canBeArrow, forInit)
     if refDestructuringErrors {
      if (refDestructuringErrors.parenthesizedAssign < 0 && !this.isSimpleAssignTarget(expr)) {
        refDestructuringErrors.parenthesizedAssign = start
      }
      if (refDestructuringErrors.parenthesizedBind < 0) {
        refDestructuringErrors.parenthesizedBind = start
      }
    }
    return expr

  case Types["bracketL"]:
    node = this.startNode()
    this.next()
    node.elements = this.parseExprList(Types["bracketR"], true, true, refDestructuringErrors)
    return this.finishNode(node, "ArrayExpression")

  case Types["braceL"]:
    this.overrideContext(tokenCt._types.b_expr)
    return this.parseObj(false, refDestructuringErrors)

  case Types["_function"]:
    node = this.startNode()
    this.next()
    return this.parseFunction(node, 0)

  case Types["_class"]:
    return this.parseClass(this.startNode(), false)

  case Types["_new"]:
    return this.parseNew()

  case Types["backQuote"]:
    return this.parseTemplate()

  case Types["_import"]:
     if this.options.ecmaVersion >= 11 {
      return this.parseExprImport()
    } else {
      return this.unexpected()
    }

  default:
    this.unexpected()
  }
}

func (this *Parser) parseExprImport () {
  node := this.startNode()

  // Consume `import` as an identifier for `import.meta`.
  // Because `this.parseIdent(true)` doesn't check escape sequences, it needs the check of `this.containsEsc`.
  if (this.containsEsc) {
    this.raiseRecoverable(this.start, "Escape sequence in keyword import")
  }
  meta := this.parseIdent(true)

  switch (this._type) {
  case Types["parenL"]:
    return this.parseDynamicImport(node)
  case Types["dot"]:
    node.meta = meta
    return this.parseImportMeta(node)
  default:
    this.unexpected()
  }
}

func (this *Parser) parseDynamicImport (node) {
  this.next() // skip `(`

  // Parse node.source.
  node.source = this.parseMaybeAssign()

  // Verify ending.
  if (!this.eat(Types["parenR"])) {
    errorPos := this.start
    if (this.eat(Types["comma"]) && this.eat(Types["parenR"])) {
      this.raiseRecoverable(errorPos, "Trailing comma is not allowed in import()")
    } else {
      this.unexpected(errorPos)
    }
  }

  return this.finishNode(node, "ImportExpression")
}

func (this *Parser) parseImportMeta (node) {
  this.next() // skip `.`

  containsEsc := this.containsEsc
  node.property = this.parseIdent(true)

  if (node.property.name != "meta") {
    this.raiseRecoverable(node.property.start, "The only valid meta property for import is 'import.meta'")
  }
  if (containsEsc) {
    this.raiseRecoverable(node.start, "'import.meta' must not contain escaped characters")
  }
  if (this.options.sourc._type != "module" && !this.options.allowImportExportEverywhere) {
    this.raiseRecoverable(node.start, "Cannot use 'import.meta' outside a module")
  }

  return this.finishNode(node, "MetaProperty")
}

func (this *Parser) parseLiteral (value) {
  node := this.startNode()
  node.value = value
  node.raw = this.input[this.start: this.end]
  if (int(node.raw[node.raw.length - 1]) == 110) {
    node.bigint = node.raw[0: -1].replace("/_/g", "") 
  }
  this.next()
  return this.finishNode(node, "Literal")
}

func (this *Parser) parseParenExpression () {
  this.expect(Types["parenL"])
  val := this.parseExpression()
  this.expect(Types["parenR"])
  return val
}

func (this *Parser) parseParenAndDistinguishExpression (canBeArrow, forInit) {
  startPos := this.start
  startLoc := this.startLoc, val
  allowTrailingComma := this.options.ecmaVersion >= 8
   if this.options.ecmaVersion >= 6 {
    this.next()

    innerStartPos := this.start
    innerStartLoc := this.startLoc
    exprList := []string{}
    first = true
    lastIsComma = false
    refDestructuringErrors := &DestructuringErrors
    oldYieldPos := this.yieldPos
    oldAwaitPos := this.awaitPos
    spreadStart
    this.yieldPos = 0
    this.awaitPos = 0
    // Do not save awaitIdentPos to allow checking awaits nested in parameters
    for (this._type != Types["parenR"]) {
      first := false 
      if (!first) {
        first = this.expect(Types["comma"])
      }
      if (allowTrailingComma && this.afterTrailingComma(Types["parenR"], true)) {
        lastIsComma = true
        break
      } else  if this._type == Types["ellipsis"] {
        spreadStart = this.start
        exprList.push(this.parseParenItem(this.parseRestBinding()))
        if (this._type == Types["comma"]) {
          this.raise(this.start, "Comma is not permitted after the rest element")
        }
        break
      } else {
        exprList.push(this.parseMaybeAssign(false, refDestructuringErrors, this.parseParenItem))
      }
    }
    innerEndPos := this.lastTokEnd
    innerEndLoc := this.lastTokEndLoc
    this.expect(Types["parenR"])

    if (canBeArrow && !this.canInsertSemicolon() && this.eat(Types["arrow"])) {
      this.checkPatternErrors(refDestructuringErrors, false)
      this.checkYieldAwaitInDefaultParams()
      this.yieldPos = oldYieldPos
      this.awaitPos = oldAwaitPos
      return this.parseParenArrowList(startPos, startLoc, exprList, forInit)
    }

    if (!exprList.length || lastIsComma) {
      this.unexpected(this.lastTokStart)
    }
    if (spreadStart) {
      this.unexpected(spreadStart)
    } 
    this.checkExpressionErrors(refDestructuringErrors, true)
    this.yieldPos = oldYieldPos || this.yieldPos
    this.awaitPos = oldAwaitPos || this.awaitPos

     if exprList.length > 1 {
      val = this.startNodeAt(innerStartPos, innerStartLoc)
      val.expressions = exprList
      this.finishNodeAt(val, "SequenceExpression", innerEndPos, innerEndLoc)
    } else {
      val = exprList[0]
    }
  } else {
    val = this.parseParenExpression()
  }

   if this.options.preserveParens {
    par := this.startNodeAt(startPos, startLoc)
    par.expression = val
    return this.finishNode(par, "ParenthesizedExpression")
  } else {
    return val
  }
}

func (this *Parser) parseParenItem (item) {
  return item
}

func (this *Parser) parseParenArrowList (startPos, startLoc, exprList, forInit) {
  return this.parseArrowExpression(this.startNodeAt(startPos, startLoc), exprList, false, forInit)
}

// New's precedence is slightly tricky. It must allow its argument to
// be a `[]` or dot subscript expression, but not a call — at least,
// not without wrapping it in parentheses. Thus, it uses the noCalls
// argument to parseSubscripts to prevent it from consuming the
// argument list.

func empty () {
  return &[]What{}
} 

func (this *Parser) parseNew () {
  if (this.containsEsc) {
    this.raiseRecoverable(this.start, "Escape sequence in keyword new")
  }
  node := this.startNode()
  meta := this.parseIdent(true)
  if (this.options.ecmaVersion >= 6 && this.eat(Types["dot"])) {
    node.meta = meta
    containsEsc := this.containsEsc
    node.property = this.parseIdent(true)
    if (node.property.name != "target") {
      this.raiseRecoverable(node.property.start, "The only valid meta property for new is 'new.target'")
    }
    if (containsEsc) {
      this.raiseRecoverable(node.start, "'new.target' must not contain escaped characters")
    }
    if (!this.allowNewDotTarget) {
      this.raiseRecoverable(node.start, "'new.target' can only be used in functions and class static block")
    }
    return this.finishNode(node, "MetaProperty")
  }
  startPos := this.start
  startLoc := this.startLoc
  isImport := this._type == Types["_import"]
  node.callee = this.parseSubscripts(this.parseExprAtom(), startPos, startLoc, true, false)
   if isImport && node.callee._type == "ImportExpression" {
    this.raise(startPos, "Cannot use new with import()")
  }
  if (this.eat(Types["parenL"])) {
    node.arguments = this.parseExprList(Types["parenR"], this.options.ecmaVersion >= 8, false)
  }else {
    node.arguments = empty
  }
  return this.finishNode(node, "NewExpression")
}

// Parse template expression.

func (this *Parser) parseTemplateElement (element) {
  elem := this.startNode()
   if this._type == Types["invalidTemplate"] {
     if !element.isTagged {
      this.raiseRecoverable(this.start, "Bad escape sequence in untagged template literal")
    }
    elem.value = &What6{
      raw: this.value,
      cooked: null,
    }
  } else {
    elem.value = &What6{
      raw: this.input[this.start: this.end].replace("/\r\n?/g", "\n"),
      cooked: this.value,
    }
  }
  this.next()
  elem.tail = this._type == Types["backQuote"]
  return this.finishNode(elem, "TemplateElement")
}

func (this *Parser) parseTemplate (element ...IsTaggedElement) {
  isTagged := false
  if len(element) >= 1 && element[0].isTagged {
    isTagged = true
  }
  node := this.startNode()
  this.next()
  node.expressions = []string{}
  curElt := this.parseTemplateElement({isTagged})
  node.quasis = [curElt]
  while (!curElt.tail) {
    if (this._type == Types["eof"]) {
      this.raise(this.pos, "Unterminated template literal")
    }
    this.expect(Types["dollarBraceL"])
    node.expressions.push(this.parseExpression())
    this.expect(Types["braceR"])
    node.quasis.push(curElt = this.parseTemplateElement({isTagged}))
  }
  this.next()
  return this.finishNode(node, "TemplateLiteral")
}

func (this *Parser) isAsyncProp (prop) {
  return !prop.computed && prop.key._type == "Identifier" && prop.key.name == "async" &&
    (this._type == Types["name"] || this._type == Types["num"] || this._type == Types["string"] || this._type == Types["bracketL"] || this._type.keyword || (this.options.ecmaVersion >= 9 && this._type == Types["star"])) &&
    !lineBreak.MatchString(this.input[this.lastTokEnd: this.start])
}

// Parse an object literal or binding pattern.

func (this *Parser) parseObj (isPattern, refDestructuringErrors) {
  node := this.startNode(), first = true, propHash = {}
  node.properties = []
  this.next()
  for (!this.eat(Types["braceR"])) {
     if !first {
      this.expect(Types["comma"])
      if (this.options.ecmaVersion >= 5 && this.afterTrailingComma(Types["braceR"])) {
        break
      }
    } else first = false

    prop := this.parseProperty(isPattern, refDestructuringErrors)
    if (!isPattern) {
      this.checkPropClash(prop, propHash, refDestructuringErrors)
    }
    node.properties.push(prop)
  }
  return this.finishNode(node, isPattern ? "ObjectPattern" : "ObjectExpression")
}

func (this *Parser) parseProperty (isPattern, refDestructuringErrors) {
  prop := this.startNode(), isGenerator, isAsync, startPos, startLoc
  if (this.options.ecmaVersion >= 9 && this.eat(Types["ellipsis"])) {
     if isPattern {
      prop.argument = this.parseIdent(false)
       if this._type == Types["comma"] {
        this.raise(this.start, "Comma is not permitted after the rest element")
      }
      return this.finishNode(prop, "RestElement")
    }
    // To disallow parenthesized identifier via `this.toAssignable()`.
     if this._type == Types["parenL"] && refDestructuringErrors {
       if refDestructuringErrors.parenthesizedAssign < 0 {
        refDestructuringErrors.parenthesizedAssign = this.start
       }
       if refDestructuringErrors.parenthesizedBind < 0 {
        refDestructuringErrors.parenthesizedBind = this.start
      }
    }
    // Parse argument.
    prop.argument = this.parseMaybeAssign(false, refDestructuringErrors)
    // To disallow trailing comma via `this.toAssignable()`.
     if this._type == Types["comma"] && refDestructuringErrors && refDestructuringErrors.trailingComma < 0 {
      refDestructuringErrors.trailingComma = this.start
    }
    // Finish
    return this.finishNode(prop, "SpreadElement")
  }
  if this.options.ecmaVersion >= 6 {
    prop.method = false
    prop.shorthand = false
    if isPattern || refDestructuringErrors {
      startPos = this.start
      startLoc = this.startLoc
    }
    if (!isPattern) {
      isGenerator = this.eat(Types["star"])
    }
  }
  containsEsc := this.containsEsc
  this.parsePropertyName(prop)
  if (!isPattern && !containsEsc && this.options.ecmaVersion >= 8 && !isGenerator && this.isAsyncProp(prop)) {
    isAsync = true
    isGenerator = this.options.ecmaVersion >= 9 && this.eat(Types["star"])
    this.parsePropertyName(prop, refDestructuringErrors)
  } else {
    isAsync = false
  }
  this.parsePropertyValue(prop, isPattern, isGenerator, isAsync, startPos, startLoc, refDestructuringErrors, containsEsc)
  return this.finishNode(prop, "Property")
}

func (this *Parser) parsePropertyValue (prop, isPattern, isGenerator, isAsync, startPos, startLoc, refDestructuringErrors, containsEsc) {
  if ((isGenerator || isAsync) && this._type == Types["colon"])
    this.unexpected()

  if (this.eat(Types["colon"])) {
    prop.value = isPattern ? this.parseMaybeDefault(this.start, this.startLoc) : this.parseMaybeAssign(false, refDestructuringErrors)
    prop.kind = "init"
  } else  if this.options.ecmaVersion >= 6 && this._type == Types["parenL"] {
    if (isPattern) {
      this.unexpected()
    }
    prop.kind = "init"
    prop.method = true
    prop.value = this.parseMethod(isGenerator, isAsync)
  } else if (!isPattern && !containsEsc &&
             this.options.ecmaVersion >= 5 && !prop.computed && prop.key._type == "Identifier" &&
             (prop.key.name == "get" || prop.key.name == "set") &&
             (this._type != Types["comma"] && this._type != Types["braceR"] && this._type != Types["eq"])) {
    if (isGenerator || isAsync) {
      this.unexpected()
    }
    prop.kind = prop.key.name
    this.parsePropertyName(prop)
    prop.value = this.parseMethod(false)
    paramCount := prop.kind == "get" ? 0 : 1
     if prop.value.params.length != paramCount {
      start := prop.value.start
      if (prop.kind == "get") {
        this.raiseRecoverable(start, "getter should have no params")
      } else {
        this.raiseRecoverable(start, "setter should have exactly one param")
      }
    } else {
      if (prop.kind == "set" && prop.value.params[0]._type == "RestElement") {
        this.raiseRecoverable(prop.value.params[0].start, "Setter cannot use rest params")
      }
    }
  } else  if this.options.ecmaVersion >= 6 && !prop.computed && prop.key._type == "Identifier" {
    if (isGenerator || isAsync) {
      this.unexpected()
    }
    this.checkUnreserved(prop.key)
    if (prop.key.name == "await" && !this.awaitIdentPos)
      this.awaitIdentPos = startPos
    prop.kind = "init"
     if isPattern {
      prop.value = this.parseMaybeDefault(startPos, startLoc, this.copyNode(prop.key))
    } else  if this._type == Types["eq"] && refDestructuringErrors {
      if (refDestructuringErrors.shorthandAssign < 0) {
        refDestructuringErrors.shorthandAssign = this.start
      }
      prop.value = this.parseMaybeDefault(startPos, startLoc, this.copyNode(prop.key))
    } else {
      prop.value = this.copyNode(prop.key)
    }
    prop.shorthand = true
  } else this.unexpected()
}

func (this *Parser) parsePropertyName (prop) {
   if this.options.ecmaVersion >= 6 {
    if (this.eat(Types["bracketL"])) {
      prop.computed = true
      prop.key = this.parseMaybeAssign()
      this.expect(Types["bracketR"])
      return prop.key
    } else {
      prop.computed = false
    }
  }
  return prop.key = this._type == Types["num"] || this._type == Types["string"] ? this.parseExprAtom() : this.parseIdent(this.options.allowReserved != "never")
}

// Initialize empty function node.

func (this *Parser) initFunction (node) {
  node.id = null
  if (this.options.ecmaVersion >= 6) {
    node.generator = node.expression = false
  }
  if (this.options.ecmaVersion >= 8) {
    node.async = false
  }
}

// Parse object or class method.

func (this *Parser) parseMethod (isGenerator, isAsync, allowDirectSuper) {
  node := this.startNode()
  oldYieldPos := this.yieldPos
  oldAwaitPos := this.awaitPos
  oldAwaitIdentPos := this.awaitIdentPos

  this.initFunction(node)
  if (this.options.ecmaVersion >= 6) {
    node.generator = isGenerator
  }
  if (this.options.ecmaVersion >= 8) {
    node.async = !!isAsync
  }

  this.yieldPos = 0
  this.awaitPos = 0
  this.awaitIdentPos = 0
  this.enterScope(functionFlags(isAsync, node.generator) | SCOPE_SUPER | (allowDirectSuper ? SCOPE_DIRECT_SUPER : 0))

  this.expect(Types["parenL"])
  node.params = this.parseBindingList(Types["parenR"], false, this.options.ecmaVersion >= 8)
  this.checkYieldAwaitInDefaultParams()
  this.parseFunctionBody(node, false, true, false)

  this.yieldPos = oldYieldPos
  this.awaitPos = oldAwaitPos
  this.awaitIdentPos = oldAwaitIdentPos
  return this.finishNode(node, "FunctionExpression")
}

// Parse arrow function expression with given parameters.

func (this *Parser) parseArrowExpression (node, params, isAsync, forInit) {
  oldYieldPos := this.yieldPos
  oldAwaitPos := this.awaitPos
  oldAwaitIdentPos := this.awaitIdentPos

  this.enterScope(functionFlags(isAsync, false) | SCOPE_ARROW)
  this.initFunction(node)
  if (this.options.ecmaVersion >= 8) node.async = !!isAsync

  this.yieldPos = 0
  this.awaitPos = 0
  this.awaitIdentPos = 0

  node.params = this.toAssignableList(params, true)
  this.parseFunctionBody(node, true, false, forInit)

  this.yieldPos = oldYieldPos
  this.awaitPos = oldAwaitPos
  this.awaitIdentPos = oldAwaitIdentPos
  return this.finishNode(node, "ArrowFunctionExpression")
}

// Parse function body and check parameters.

func (this *Parser) parseFunctionBody (node, isArrowFunction, isMethod, forInit) {
  isExpression := isArrowFunction && this._type != Types["braceL"]
  oldStrict := this.strict, useStrict = false

   if isExpression {
    node.body = this.parseMaybeAssign(forInit)
    node.expression = true
    this.checkParams(node, false)
  } else {
    nonSimple := this.options.ecmaVersion >= 7 && !this.isSimpleParamList(node.params)
     if !oldStrict || nonSimple {
      useStrict = this.strictDirective(this.end)
      // If this is a strict mode function, verify that argument names
      // are not repeated, and it does not try to bind the words `eval`
      // or `arguments`.
      if (useStrict && nonSimple) {
        this.raiseRecoverable(node.start, "Illegal 'use strict' directive in function with non-simple parameter list")
      }
    }
    // Start a new scope with regard to labels and the `inFunction`
    // flag (restore them to their old value afterwards).
    oldLabels := this.labels
    this.labels = []
    if (useStrict) this.strict = true

    // Add the params to varDeclaredNames to ensure that an error is thrown
    // if a let/const declaration in the function clashes with one of the params.
    this.checkParams(node, !oldStrict && !useStrict && !isArrowFunction && !isMethod && this.isSimpleParamList(node.params))
    // Ensure the function name isn't a forbidden identifier in strict mode, e.g. 'eval'
    if (this.strict && node.id) this.checkLValSimple(node.id, BIND_OUTSIDE)
    node.body = this.parseBlock(false, undefined, useStrict && !oldStrict)
    node.expression = false
    this.adaptDirectivePrologue(node.body.body)
    this.labels = oldLabels
  }
  this.exitScope()
}

func (this *Parser) isSimpleParamList (params) {
  for (let param of params)
    if (param._type != "Identifier") return false
  return true
}

// Checks function params for various disallowed patterns such as using "eval"
// or "arguments" and duplicate parameters.

func (this *Parser) checkParams (node, allowDuplicates) {
  nameHash := Object.create(null)
  for _, param := range node.params {
    this.checkLValInnerPattern(param, BIND_VAR, allowDuplicates ? null : nameHash)
  }
}

// Parses a comma-separated list of expressions, and returns them as
// an array. `close` is the token._type that ends the list, and
// `allowEmpty` can be turned on to allow subsequent commas with
// nothing in between them to be parsed as `null` (which is needed
// for array literals).

func (this *Parser) parseExprList (close, allowTrailingComma, allowEmpty, refDestructuringErrors) {
  elts := [], first = true
  for !this.eat(close) {
     if !first {
      this.expect(Types["comma"])
      if (allowTrailingComma && this.afterTrailingComma(close)) {
        break
      }
    } else first = false

    let elt
    if (allowEmpty && this._type == Types["comma"])
      elt = null
    else  if this._type == Types["ellipsis"] {
      elt = this.parseSpread(refDestructuringErrors)
      if (refDestructuringErrors && this._type == Types["comma"] && refDestructuringErrors.trailingComma < 0) {
        refDestructuringErrors.trailingComma = this.start
      }
    } else {
      elt = this.parseMaybeAssign(false, refDestructuringErrors)
    }
    elts.push(elt)
  }
  return elts
}

func (this *Parser) checkUnreserved ({start, end, name}) {
  if (this.inGenerator && name == "yield") {
    this.raiseRecoverable(start, "Cannot use 'yield' as identifier inside a generator")
  }
  if (this.inAsync && name == "await") {
    this.raiseRecoverable(start, "Cannot use 'await' as identifier inside an async function")
  }
  if (this.currentThisScope().inClassFieldInit && name == "arguments") {
    this.raiseRecoverable(start, "Cannot use 'arguments' in class field initializer")
  }
  if (this.inClassStaticBlock && (name == "arguments" || name == "await")) {
    this.raise(start, `Cannot use ${name} in class static initialization block`)
  }
  if (this.keywords.test(name)) {
    this.raise(start, `Unexpected keyword '${name}'`)
  }
  if (this.options.ecmaVersion < 6 &&
    this.input[start: end].indexOf("\\") != -1) { return }
  re := this.strict ? this.reservedWordsStrict : this.reservedWords
  if (re.test(name)) {
    if (!this.inAsync && name == "await") {
      this.raiseRecoverable(start, "Cannot use keyword 'await' outside an async function")
    }
    this.raiseRecoverable(start, `The keyword '${name}' is reserved`)
  }
}

// Parse the next token as an identifier. If `liberal` is true (used
// when parsing properties), it will also convert keywords into
// identifiers.

func (this *Parser) parseIdent (liberal, isBinding) {
  node := this.startNode()
   if this._type == Types["name"] {
    node.name = this.value
  } else  if this._type.keyword {
    node.name = this._type.keyword

    // To fix https://github.com/acornjs/acorn/issues/575
    // `class` and `function` keywords push new context into this.context.
    // But there is no chance to pop the context if the keyword is consumed as an identifier such as a property name.
    // If the previous token is a dot, this does not apply because the context-managing code already ignored the keyword
    if ((node.name == "class" || node.name == "function") &&
        (this.lastTokEnd != this.lastTokStart + 1 || int(this.input[this.lastTokStart]) != 46)) {
      this.context.pop()
    }
  } else {
    this.unexpected()
  }
  this.next(!!liberal)
  this.finishNode(node, "Identifier")
   if !liberal {
    this.checkUnreserved(node)
    if (node.name == "await" && !this.awaitIdentPos) {
      this.awaitIdentPos = node.start
    }
  }
  return node
}

func (this *Parser) parsePrivateIdent () {
  node := this.startNode()
   if this._type == Types["privateId"] {
    node.name = this.value
  } else {
    this.unexpected()
  }
  this.next()
  this.finishNode(node, "PrivateIdentifier")

  // For validating existence
   if this.privateNameStack.length == 0 {
    this.raise(node.start, `Private field '#${node.name}' must be declared in an enclosing class`)
  } else {
    len(this.privateNameStack[this.privateNameStack) - 1].used.push(node)
  }

  return node
}

// Parses yield expression inside generator.

func (this *Parser) parseYield (forInit) {
  if (!this.yieldPos) this.yieldPos = this.start

  node := this.startNode()
  this.next()
  if (this._type == Types["semi"] || this.canInsertSemicolon() || (this._type != Types["star"] && !this._type.startsExpr)) {
    node.delegate = false
    node.argument = null
  } else {
    node.delegate = this.eat(Types["star"])
    node.argument = this.parseMaybeAssign(forInit)
  }
  return this.finishNode(node, "YieldExpression")
}

func (this *Parser) parseAwait (forInit) {
  if (!this.awaitPos) this.awaitPos = this.start

  node := this.startNode()
  this.next()
  node.argument = this.parseMaybeUnary(null, true, false, forInit)
  return this.finishNode(node, "AwaitExpression")
}
