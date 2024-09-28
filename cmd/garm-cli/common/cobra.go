package common

import "fmt"

type OutputFormat string

const (
	OutputFormatTable OutputFormat = "table"
	OutputFormatJSON  OutputFormat = "json"
)

func (o *OutputFormat) String() string {
	if o == nil {
		return ""
	}
	return string(*o)
}

func (o *OutputFormat) Set(value string) error {
	switch value {
	case "table", "json":
		*o = OutputFormat(value)
	default:
		return fmt.Errorf("allowed formats are: json, table")
	}
	return nil
}

func (o *OutputFormat) Type() string {
	return "string"
}
