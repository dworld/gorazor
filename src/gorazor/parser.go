package gorazor

import (
	"fmt"
	"regexp"
	"strings"
)

//------------------------------ Parser ------------------------------//
const (
	UNK = iota
	PRG
	MKP
	BLK
	EXP
)

var PAIRS = map[int]int{
	AT_STAR_OPEN:    AT_STAR_CLOSE,
	BRACE_OPEN:      BRACE_CLOSE,
	DOUBLE_QUOTE:    DOUBLE_QUOTE,
	HARD_PAREN_OPEN: HARD_PAREN_CLOSE,
	PAREN_OPEN:      PAREN_CLOSE,
	SINGLE_QUOTE:    SINGLE_QUOTE,
	AT_COLON:        NEWLINE,
	FORWARD_SLASH:   FORWARD_SLASH,
}

type Ast struct {
	Parent   *Ast
	Children []interface{}
	Mode     int
	TagName  string
}

func (ast *Ast) ModeStr() string {
	switch ast.Mode {
	case PRG:
		return "PROGRAM"
	case MKP:
		return "MARKUP"
	case BLK:
		return "BLOCK"
	case EXP:
		return "EXP"
	default:
		return "UNDEF"
	}
	return "UNDEF"
}

func (ast *Ast) check() {
	if len(ast.Children) >= 100000 {
		panic("Maximum number of elements exceeded.")
	}
}

func (ast *Ast) addChild(child interface{}) {
	ast.Children = append(ast.Children, child)
	ast.check()
	if _a, ok := child.(*Ast); ok {
		_a.Parent = ast
	}
}

func (ast *Ast) addChildren(children []Token) {
	for _, c := range children {
		ast.addChild(c)
	}
}

func (ast *Ast) addAst(_ast *Ast) {
	c := _ast
	for {
		if len(c.Children) != 1 {
			break
		}
		first := c.Children[0]
		if _, ok := first.(*Ast); !ok {
			break
		}
		c = first.(*Ast)
	}
	if c.Mode != PRG {
		ast.addChild(c)
	} else {
		for _, x := range c.Children {
			ast.addChild(x)
		}
	}
}

func (ast *Ast) popChild() {
	l := len(ast.Children)
	if l == 0 {
		return
	}
	ast.Children = ast.Children[:l-1]
}

func (ast *Ast) root() *Ast {
	p := ast
	pp := ast.Parent
	for {
		if p == pp || pp == nil {
			return p
		}
		b := pp
		pp = p.Parent
		p = b
	}
	return nil
}

func (ast *Ast) beget(mode int, tag string) *Ast {
	child := &Ast{ast, []interface{}{}, mode, tag}
	ast.addChild(child)
	return child
}

func (ast *Ast) closest(mode int, tag string) *Ast {
	p := ast
	for {
		if p.TagName != tag && p.Parent != nil {
			p = p.Parent
		} else {
			break
		}
	}
	return p
}

func (ast *Ast) hasNonExp() bool {
	if ast.Mode != EXP {
		return true
	} else {
		for _, c := range ast.Children {
			if v, ok := c.(*Ast); ok {
				if v.hasNonExp() {
					return true
				}
			}
			return false
		}
	}
	return false
}

func (ast *Ast) debug(depth int, max int) {
	if depth >= max {
		return
	}
	for i := 0; i < depth; i++ {
		fmt.Printf("%c", '-')
	}
	fmt.Printf("TagName: %s Mode: %s Children: %d [[ \n", ast.TagName, ast.ModeStr(), len(ast.Children))
	for _, a := range ast.Children {
		//fmt.Printf("(%d)", idx)
		if _, ok := a.(*Ast); ok {
			b := (*Ast)(a.(*Ast))
			b.debug(depth+1, max)
		} else {
			if depth+1 < max {
				aa := (Token)(a.(Token))
				for i := 0; i < depth+1; i++ {
					fmt.Printf("%c", '-')
				}
				aa.P()
			}
		}
	}
	for i := 0; i < depth; i++ {
		fmt.Printf("%c", '-')
	}

	fmt.Println("]]")
}

type Parser struct {
	ast         *Ast
	tokens      []Token
	preTokens   []Token
	inComment   bool
	saveTextTag bool
	initMode    int
}

func (parser *Parser) prevToken(idx int) *Token {
	l := len(parser.preTokens)
	if l < idx+1 {
		return nil
	}
	return &(parser.preTokens[l-1-idx])
}

func (parser *Parser) deferToken(token Token) {
	parser.tokens = append([]Token{token}, parser.tokens...)
	parser.preTokens = parser.preTokens[:len(parser.preTokens)-1]
}

func (parser *Parser) peekToken(idx int) *Token {
	if len(parser.tokens) <= idx {
		return nil
	}
	return &(parser.tokens[idx])
}

func (parser *Parser) nextToken() Token {
	t := parser.peekToken(0)
	if t != nil {
		parser.tokens = parser.tokens[1:]
	}
	return *t
}

func (parser *Parser) skipToken() {
	parser.tokens = parser.tokens[1:]
}

func regMatch(reg string, text string) (string, error) {
	regc, err := regexp.Compile(reg)
	if err != nil {
		panic(err)
		return "", err
	}
	found := regc.FindIndex([]byte(text))
	if found != nil {
		return text[found[0]:found[1]], nil
	}
	return "", nil
}

func (parser *Parser) advanceUntilNot(tokenType int) []Token {
	res := []Token{}
	for {
		t := parser.peekToken(0)
		if t != nil && t.Type == tokenType {
			res = append(res, parser.nextToken())
		} else {
			break
		}
	}
	return res
}

func (parser *Parser) advanceUntil(token Token, start, end, startEsc, endEsc int) []Token {
	var prev *Token = nil
	next := &token
	res := []Token{}
	nstart := 0
	nend := 0
	for {
		if next.Type == start {
			if (prev != nil && prev.Type != startEsc && start != end) || prev == nil {
				nstart++
			} else if start == end && prev.Type != startEsc {
				nend++
			}
		} else if next.Type == end {
			nend++
			if prev != nil && prev.Type == endEsc {
				nend--
			}
		}
		res = append(res, *next)
		if nstart == nend {
			break
		}
		prev = next
		next = parser.peekToken(0)
		if next == nil {
			panic("UNMATCHED")
		}
		parser.nextToken()
	}
	return res
}

func (parser *Parser) subParse(token Token, modeOpen int, includeDelim bool) {
	subTokens := parser.advanceUntil(token, token.Type, PAIRS[token.Type], -1, AT)
	subTokens = subTokens[1:]
	closer := subTokens[len(subTokens)-1]
	subTokens = subTokens[:len(subTokens)-1]
	if !includeDelim {
		parser.ast.addChild(token)

	}
	_parser := &Parser{&Ast{}, subTokens, []Token{}, false, false, modeOpen}
	_parser.Run()
	if includeDelim {
		_parser.ast.Children = append([]interface{}{token}, _parser.ast.Children...)
		_parser.ast.addChild(closer)
	}
	//_parser.ast.debug(0)
	parser.ast.addAst(_parser.ast)
	if !includeDelim {
		parser.ast.addChild(closer)
	}
}

func (parser *Parser) handleMKP(token Token) {
	next := parser.peekToken(0)
	//nnext := parser.peekToken(1)
	switch token.Type {
	case AT_STAR_OPEN:
		parser.advanceUntil(token, AT_STAR_OPEN, AT_STAR_CLOSE, AT, AT)

	case AT:
		if next != nil {
			switch next.Type {
			case PAREN_OPEN, IDENTIFIER:
				if len(parser.ast.Children) == 0 {
					parser.ast = parser.ast.Parent
					parser.ast.popChild() //remove empty MKP block
				}
				parser.ast = parser.ast.beget(EXP, "")

			case KEYWORD, FUNCTION, BRACE_OPEN: //BLK
				if len(parser.ast.Children) == 0 {
					parser.ast = parser.ast.Parent
					parser.ast.popChild()
				}
				parser.ast = parser.ast.beget(BLK, "")

			case AT, AT_COLON:
				//we want to keep the token, but remove it's special meanning
				next.Type = CONTENT //Modify from a pointer, work?
				parser.ast.addChild(parser.nextToken())
			default:
				parser.ast.addChild(parser.nextToken())
			}
		}

	case TEXT_TAG_OPEN, HTML_TAG_OPEN:
		tagName, _ := regMatch(`(?i)(^<([^\/ >]+))`, token.Text)
		tagName = strings.Replace(tagName, "<", "", -1)
		//TODO
		if parser.ast.TagName != "" {
			parser.ast = parser.ast.beget(MKP, tagName)
		} else {
			parser.ast.TagName = tagName
		}
		if token.Type == HTML_TAG_OPEN || parser.saveTextTag {
			parser.ast.addChild(token)
		}

	case TEXT_TAG_CLOSE, HTML_TAG_CLOSE:
		tagName, _ := regMatch(`(?i)^<\/([^>]+)`, token.Text)
		tagName = strings.Replace(tagName, "</", "", -1)
		//TODO
		opener := parser.ast.closest(MKP, tagName)
		if opener.TagName != tagName { //unmatched
			panic("UNMATCHED!")
		} else {
			parser.ast = opener
		}
		if token.Type == HTML_TAG_CLOSE || parser.saveTextTag {
			parser.ast.addChild(token)
		}

		// vash.js have bug here, we should skip current MKP,
		// so that we can keep in a right hierarchy
		if parser.ast.Parent != nil && parser.ast.Parent.Mode == BLK {
			parser.ast = parser.ast.Parent
		}

	case HTML_TAG_VOID_CLOSE:
		parser.ast.addChild(token)
		parser.ast = parser.ast.Parent

	case BACKSLASH:
		token.Text += "\\"
		parser.ast.addChild(token)
	default:
		parser.ast.addChild(token)
	}
}

func (parser *Parser) handleBLK(token Token) {
	next := parser.peekToken(0)
	switch token.Type {
	case AT:
		if (next.Type != AT) && (!parser.inComment) {
			parser.deferToken(token)
			parser.ast = parser.ast.beget(MKP, "")
		} else {
			next.Type = CONTENT
			parser.ast.addChild(*next)
			parser.skipToken()
		}

	case AT_STAR_OPEN:
		parser.advanceUntil(token, AT_STAR_OPEN, AT_STAR_CLOSE, AT, AT)

	case AT_COLON:
		parser.subParse(token, MKP, true)

	case TEXT_TAG_OPEN, TEXT_TAG_CLOSE, HTML_TAG_OPEN, HTML_TAG_CLOSE:
		parser.ast = parser.ast.beget(MKP, "")
		parser.deferToken(token)

	case FORWARD_SLASH, SINGLE_QUOTE, DOUBLE_QUOTE:
		if token.Type == FORWARD_SLASH && next != nil && next.Type == FORWARD_SLASH {
			parser.inComment = true
		}
		if !parser.inComment {
			subTokens := parser.advanceUntil(token, token.Type,
				PAIRS[token.Type],
				BACKSLASH,
				BACKSLASH)
			for idx, _ := range subTokens {
				if subTokens[idx].Type == AT {
					subTokens[idx].Type = CONTENT
				}
			}
			parser.ast.addChildren(subTokens)
		} else {
			parser.ast.addChild(token)
		}

	case NEWLINE:
		if parser.inComment {
			parser.inComment = false
		}
		parser.ast.addChild(token)

	case BRACE_OPEN, PAREN_OPEN:
		subMode := BLK
		if false && token.Type == BRACE_OPEN { //TODO
			subMode = MKP
		}
		parser.subParse(token, subMode, false)
		subTokens := parser.advanceUntilNot(WHITESPACE)
		next := parser.peekToken(0)
		if next != nil && next.Type != KEYWORD &&
			next.Type != FUNCTION && next.Type != BRACE_OPEN &&
			token.Type != PAREN_OPEN {
			parser.tokens = append(parser.tokens, subTokens...)
			parser.ast = parser.ast.Parent
		} else {
			parser.ast.addChildren(subTokens)
		}
	default:
		parser.ast.addChild(token)
	}
}

func (parser *Parser) handleEXP(token Token) {
	switch token.Type {
	case KEYWORD, FUNCTION:
		parser.ast = parser.ast.beget(BLK, "")
		parser.deferToken(token)

	case WHITESPACE, LOGICAL, ASSIGN_OPERATOR, OPERATOR, NUMERIC_CONTENT:
		if parser.ast.Parent != nil && parser.ast.Parent.Mode == EXP {
			parser.ast.addChild(token)
		} else {
			parser.ast = parser.ast.Parent
			parser.deferToken(token)
		}
	case IDENTIFIER:
		parser.ast.addChild(token)

	case SINGLE_QUOTE, DOUBLE_QUOTE:
		//TODO
		if parser.ast.Parent != nil && parser.ast.Parent.Mode == EXP {
			subTokens := parser.advanceUntil(token, token.Type,
				PAIRS[token.Type], BACKSLASH, BACKSLASH)
			parser.ast.addChildren(subTokens)
		} else {
			parser.ast = parser.ast.Parent
			parser.deferToken(token)
		}

	case HARD_PAREN_OPEN, PAREN_OPEN:
		prev := parser.prevToken(0)
		next := parser.peekToken(0)
		if token.Type == HARD_PAREN_OPEN && next.Type == HARD_PAREN_CLOSE {
			// likely just [], which is not likely valid outside of EXP
			parser.deferToken(token)
			parser.ast = parser.ast.Parent
			break
		}
		parser.subParse(token, EXP, false)
		if (prev != nil && prev.Type == AT) || (next != nil && next.Type == IDENTIFIER) {
			parser.ast = parser.ast.Parent
		}

	case BRACE_OPEN:
		parser.deferToken(token)
		parser.ast = parser.ast.beget(BLK, "")

	case PERIOD:
		next := parser.peekToken(0)
		if next != nil && (next.Type == IDENTIFIER || next.Type == KEYWORD ||
			next.Type == FUNCTION || next.Type == PERIOD ||
			(parser.ast.Parent != nil && parser.ast.Parent.Mode == EXP)) {
			parser.ast.addChild(token)
		} else {
			parser.ast = parser.ast.Parent
			parser.deferToken(token)
		}
	default:
		if parser.ast.Parent != nil && parser.ast.Parent.Mode != EXP {
			parser.ast = parser.ast.Parent
			parser.deferToken(token)
		} else {
			parser.ast.addChild(token)
		}
	}
}

func (parser *Parser) Run() (err error) {
	curr := Token{"UNDEF", "UNDEF", UNDEF, 0, 0}
	parser.ast.Mode = PRG
	for {
		if len(parser.tokens) == 0 {
			break
		}
		parser.preTokens = append(parser.preTokens, curr)
		curr = parser.nextToken()
		if parser.ast.Mode == PRG {
			init := parser.initMode
			if init == UNK {
				init = MKP
			}
			parser.ast = parser.ast.beget(init, "")
			if parser.initMode == EXP {
				parser.ast = parser.ast.beget(EXP, "")
			}
		}
		switch parser.ast.Mode {
		case MKP:
			parser.handleMKP(curr)
		case BLK:
			parser.handleBLK(curr)
		case EXP:
			parser.handleEXP(curr)
		}
	}

	parser.ast = parser.ast.root()
	return nil
}
