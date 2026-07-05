package parser

import "fmt"

type DefaultParser struct{}

func (p *DefaultParser) Parse(rawMsg string, result any) error {
	strPtr, ok := result.(*string)
	if !ok {
		return fmt.Errorf("result must be a string ptr")
	}
	*strPtr = rawMsg
	return nil
}
