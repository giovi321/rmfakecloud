package templates

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Values in a template are either numbers or short expressions over the
// template size and the constants declared before them, such as
// "templateWidth > templateHeight ? 26 : 19.5". This is the whole language:
// numbers, names, the four arithmetic operators, comparisons, parentheses and
// the conditional.

type tokenKind int

const (
	tokenEnd tokenKind = iota
	tokenNumber
	tokenName
	tokenOperator
)

type token struct {
	kind  tokenKind
	text  string
	value float64
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	runes := []rune(input)

	for i := 0; i < len(runes); {
		r := runes[i]

		switch {
		case unicode.IsSpace(r):
			i++

		case unicode.IsDigit(r) || r == '.':
			start := i
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				i++
			}
			text := string(runes[start:i])
			value, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, fmt.Errorf("%q is not a number", text)
			}
			tokens = append(tokens, token{kind: tokenNumber, text: text, value: value})

		case unicode.IsLetter(r) || r == '_':
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			tokens = append(tokens, token{kind: tokenName, text: string(runes[start:i])})

		default:
			// Two character operators first, so <= does not read as < then =.
			if i+1 < len(runes) {
				pair := string(runes[i : i+2])
				if pair == "<=" || pair == ">=" || pair == "==" || pair == "!=" || pair == "&&" || pair == "||" {
					tokens = append(tokens, token{kind: tokenOperator, text: pair})
					i += 2
					continue
				}
			}
			if !strings.ContainsRune("+-*/()<>?:", r) {
				return nil, fmt.Errorf("unexpected character %q", string(r))
			}
			tokens = append(tokens, token{kind: tokenOperator, text: string(r)})
			i++
		}
	}

	return append(tokens, token{kind: tokenEnd}), nil
}

type parser struct {
	tokens []token
	at     int
	names  map[string]float64
}

func (p *parser) peek() token { return p.tokens[p.at] }

func (p *parser) accept(text string) bool {
	if p.tokens[p.at].kind == tokenOperator && p.tokens[p.at].text == text {
		p.at++
		return true
	}
	return false
}

func (p *parser) expect(text string) error {
	if !p.accept(text) {
		return fmt.Errorf("expected %q", text)
	}
	return nil
}

// The precedence chain, loosest first:
//   conditional  a ? b : c
//   logicalOr    a || b
//   logicalAnd   a && b
//   comparison   a < b
//   sum          a + b
//   product      a * b
//   unary        -a
//   atom         12, name, ( ... )

func (p *parser) conditional() (float64, error) {
	condition, err := p.logicalOr()
	if err != nil {
		return 0, err
	}

	if !p.accept("?") {
		return condition, nil
	}

	whenTrue, err := p.conditional()
	if err != nil {
		return 0, err
	}
	if err := p.expect(":"); err != nil {
		return 0, err
	}
	whenFalse, err := p.conditional()
	if err != nil {
		return 0, err
	}

	if condition != 0 {
		return whenTrue, nil
	}
	return whenFalse, nil
}

func (p *parser) logicalOr() (float64, error) {
	left, err := p.logicalAnd()
	if err != nil {
		return 0, err
	}
	for p.accept("||") {
		right, err := p.logicalAnd()
		if err != nil {
			return 0, err
		}
		left = boolean(left != 0 || right != 0)
	}
	return left, nil
}

func (p *parser) logicalAnd() (float64, error) {
	left, err := p.comparison()
	if err != nil {
		return 0, err
	}
	for p.accept("&&") {
		right, err := p.comparison()
		if err != nil {
			return 0, err
		}
		left = boolean(left != 0 && right != 0)
	}
	return left, nil
}

func (p *parser) comparison() (float64, error) {
	left, err := p.sum()
	if err != nil {
		return 0, err
	}

	for {
		operator := p.peek()
		if operator.kind != tokenOperator {
			return left, nil
		}

		switch operator.text {
		case "<", ">", "<=", ">=", "==", "!=":
			p.at++
			right, err := p.sum()
			if err != nil {
				return 0, err
			}
			left = boolean(compare(operator.text, left, right))
		default:
			return left, nil
		}
	}
}

func compare(operator string, left, right float64) bool {
	switch operator {
	case "<":
		return left < right
	case ">":
		return left > right
	case "<=":
		return left <= right
	case ">=":
		return left >= right
	case "==":
		return left == right
	case "!=":
		return left != right
	}
	return false
}

func boolean(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (p *parser) sum() (float64, error) {
	left, err := p.product()
	if err != nil {
		return 0, err
	}

	for {
		switch {
		case p.accept("+"):
			right, err := p.product()
			if err != nil {
				return 0, err
			}
			left += right
		case p.accept("-"):
			right, err := p.product()
			if err != nil {
				return 0, err
			}
			left -= right
		default:
			return left, nil
		}
	}
}

func (p *parser) product() (float64, error) {
	left, err := p.unary()
	if err != nil {
		return 0, err
	}

	for {
		switch {
		case p.accept("*"):
			right, err := p.unary()
			if err != nil {
				return 0, err
			}
			left *= right
		case p.accept("/"):
			right, err := p.unary()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		default:
			return left, nil
		}
	}
}

func (p *parser) unary() (float64, error) {
	if p.accept("-") {
		value, err := p.unary()
		return -value, err
	}
	p.accept("+")
	return p.atom()
}

func (p *parser) atom() (float64, error) {
	current := p.peek()

	switch current.kind {
	case tokenNumber:
		p.at++
		return current.value, nil

	case tokenName:
		p.at++
		value, known := p.names[current.text]
		if !known {
			return 0, fmt.Errorf("unknown name %q", current.text)
		}
		return value, nil

	case tokenOperator:
		if current.text == "(" {
			p.at++
			value, err := p.conditional()
			if err != nil {
				return 0, err
			}
			return value, p.expect(")")
		}
	}

	return 0, fmt.Errorf("unexpected %q", current.text)
}

// evaluate works out one expression against the names in scope.
func evaluate(expression string, names map[string]float64) (float64, error) {
	tokens, err := tokenize(expression)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", expression, err)
	}

	p := &parser{tokens: tokens, names: names}
	value, err := p.conditional()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", expression, err)
	}
	if p.peek().kind != tokenEnd {
		return 0, fmt.Errorf("%s: trailing %q", expression, p.peek().text)
	}
	return value, nil
}

// number reads a value that is either a plain number or an expression.
func number(value any, names map[string]float64) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case string:
		return evaluate(typed, names)
	case nil:
		return 0, nil
	}
	return 0, fmt.Errorf("%v is not a number or an expression", value)
}
