package main

type Scope struct {
	flags int
	// A list of var-declared names in the current lexical scope
	_var []string
	// A list of lexically-declared names in the current lexical scope
	lexical []string
	// A list of lexically-declared FunctionDeclaration names in the current lexical scope
	functions []string
	// A switch to disallow the identifier reference 'arguments'
	inClassFieldInit bool
}

// The functions in this module keep track of declared variables in the current scope in order to detect duplicate variable names.

func (this *Parser) enterScope(flags int) {
	this.scopeStack = append(this.scopeStack, &Scope{flags: flags})
}

func (this *Parser) exitScope() {
	this.scopeStack = this.scopeStack[1:]
}

// The spec says:
// > At the top level of a function, or script, function declarations are
// > treated like var declarations rather than like lexical declarations.
func (this *Parser) treatFunctionsAsVarInScope(scope *Scope) bool {
	return (scope.flags&SCOPE_FUNCTION != 0) || !this.inModule && (scope.flags&SCOPE_TOP != 0)
}

func (this *Parser) declareName(name string, bindingType int, pos int) {
	redeclared := false
	if bindingType == BIND_LEXICAL {
		scope := this.currentScope()
		redeclared = includes(scope.lexical, name) || includes(scope.functions, name) || includes(scope._var, name)
		scope.lexical = append(scope.lexical, name)
		if this.inModule && (scope.flags&SCOPE_TOP != 0) {
			delete(this.undefinedExports, name)
		}
	} else if bindingType == BIND_SIMPLE_CATCH {
		scope := this.currentScope()
		scope.lexical = append(scope.lexical, name)
	} else if bindingType == BIND_FUNCTION {
		scope := this.currentScope()
		if this.treatFunctionsAsVar() {
			redeclared = includes(scope.lexical, name)
		} else {
			redeclared = includes(scope.lexical, name) || includes(scope._var, name)
		}
		scope.functions = append(scope.functions, name)
	} else {
		for i := len(this.scopeStack) - 1; i >= 0; i-- {
			scope := this.scopeStack[i]
			if includes(scope.lexical, name) && !((scope.flags&SCOPE_SIMPLE_CATCH) && scope.lexical[0] == name) ||
				!this.treatFunctionsAsVarInScope(scope) && includes(scope.functions, name) {
				redeclared = true
				break
			}
			scope._var = append(scope._var, name)
			if this.inModule && (scope.flags&SCOPE_TOP != 0) {
				delete(this.undefinedExports, name)
			}
			if scope.flags&SCOPE_VAR != 0 {
				break
			}
		}
	}
	if redeclared {
		this.raiseRecoverable(pos, `Identifier '${name}' has already been declared`)
	}
}

func (this *Parser) checkLocalExport(id *Node) {
	// scope.functions must be empty as Module code is always strict.
	if includes(this.scopeStack[0].lexical, id.name) &&
		includes(this.scopeStack[0]._var, id.name) {
		this.undefinedExports[id.name] = id
	}
}

func (this *Parser) currentScope() *Scope {
	return this.scopeStack[len(this.scopeStack)-1]
}

func (this *Parser) currentVarScope() *Scope {
	for i := len(this.scopeStack) - 1; ; i-- {
		scope := this.scopeStack[i]
		if scope.flags&SCOPE_VAR != 0 {
			return scope
		}
	}
}

// Could be useful for `this`, `new.target`, `super()`, `super.property`, and `super[property]`.
func (this *Parser) currentThisScope() *Scope {
	for i := len(this.scopeStack) - 1; ; i-- {
		scope := this.scopeStack[i]
		if scope.flags&SCOPE_VAR != 0 && (scope.flags&SCOPE_ARROW == 0) {
			return scope
		}
	}
}
