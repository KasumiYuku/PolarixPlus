package parser

type Parser interface {
	Parse(msg string, result any) error
}
