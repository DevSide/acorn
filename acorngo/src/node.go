package main

/* eslint curly: "error" */
// import {Parser} from "./state.js"
// import {SourceLocation} from "./locutil.js"

type Node struct {
	id           *Node
	name         string
	_type        string
	kind         string
	value        *interface{}
	start        int
	end          int
	loc          *SourceLocation
	sourceFile   string
	sourceType   string
	source       *Node
	_range       [2]int
	body         []*Node
	init         *Node
	key          *Node
	computed     bool
	static       bool
	await        bool
	async        bool
	label        *StatementLabel
	left         *Node
	right        *Node
	discriminant *Node
	cases        []*Node
	consequent   []*Node
	alternate    *Node
	imported     *Node
	local        *Node
	test         *Node
	argument     *Node
	handler      *Node
	finalizer    *Node
	object       *Node
	expression   *Node
	generator    bool
	superClass   *Node
	exported     *Node
	declaration  *Node
	declarations []*Node
	specifiers   []*Node
	params       []*Node
	param        *Node
	block        *Node
	update       *Node
	properties   []*Node
	elements     []*Node
}

func NewNode(parser *Parser, pos int, loc *Position) *Node {
	node := &Node{_type: "", start: pos, end: 0}

	if parser.options.locations {
		node.loc = NewSourceLocation(parser, loc)
	}

	if parser.options.directSourceFile != "" {
		node.sourceFile = parser.options.directSourceFile
	}

	if parser.options.ranges {
		node._range = [2]int{pos, 0}
	}

	return node
}

// Start an AST node, attaching a start offset.

func (this *Parser) startNode() *Node {
	return NewNode(this, this.start, this.startLoc)
}

func (this *Parser) startNodeAt(pos int, loc *Position) *Node {
	return NewNode(this, pos, loc)
}

// Finish an AST node, adding `type` and `end` properties.

func finishNodeAt(parser *Parser, node *Node, _type string, pos int, loc *Position) *Node {
	node._type = _type
	node.end = pos
	if parser.options.locations {
		node.loc.end = loc
	}
	if parser.options.ranges {
		node._range[1] = pos
	}
	return node
}

func (this *Parser) finishNode(node *Node, _type string) *Node {
	return finishNodeAt(this, node, _type, this.lastTokEnd, this.lastTokEndLoc)
}

// Finish node at given position

func (this *Parser) finishNodeAt(node *Node, _type string, pos int, loc *Position) *Node {
	return finishNodeAt(this, node, _type, pos, loc)
}

func (this *Parser) copyNode(node *Node) *Node {
	newNode := *node

	return &newNode
}
