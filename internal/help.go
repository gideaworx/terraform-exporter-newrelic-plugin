package internal

import (
	"bytes"
	"errors"
	"regexp"

	"github.com/alecthomas/kong"
)

var errAvoidExit = errors.New("no error")
var helpRegex *regexp.Regexp

func init() {
	helpRegex = regexp.MustCompile(`\n\s*-h, --help\s*Show context-sensitive help.`)
}

func StringHelpPrinter(output *string) kong.HelpPrinter {
	return func(options kong.HelpOptions, ctx *kong.Context) error {
		options.NoAppSummary = true
		buf := &bytes.Buffer{}
		ctx.Stdout = buf

		err := kong.DefaultHelpPrinter(options, ctx)
		*output = buf.String()
		if err != nil {
			return err
		}

		// get rid of the -h text
		*output = helpRegex.ReplaceAllLiteralString(*output, "")
		// kong hardcodes to exit the program if there's no error, so return one we can ignore
		return errAvoidExit
	}
}

func PluginCommandHelp(intf interface{}) (string, error) {
	args := []string{"--help"}

	helpText := ""
	var k *kong.Kong
	k, err := kong.New(intf, kong.Help(StringHelpPrinter(&helpText)))
	if err != nil {
		return helpText, err
	}

	_, err = k.Parse(args)
	if err != nil && errors.Is(err, errAvoidExit) {
		err = nil
	}

	return helpText, err
}
