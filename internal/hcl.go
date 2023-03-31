package internal

import (
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func CreateHeredoc(text string, heredocMarker string, escapeSequences bool) hclwrite.Tokens {
	if escapeSequences {
		text = strings.ReplaceAll(text, "${", "$${")
		text = strings.ReplaceAll(text, "%{", "%%{")
	}

	nlWithIndent := &hclwrite.Token{
		Type:  hclsyntax.TokenNewline,
		Bytes: []byte{'\n'},
	}

	tokens := hclwrite.Tokens{
		{
			Type:  hclsyntax.TokenOHeredoc,
			Bytes: []byte("<<" + heredocMarker),
		},
	}

	if strings.HasPrefix(heredocMarker, "-") {
		tokens = append(tokens, nlWithIndent)
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			tokens = append(tokens, &hclwrite.Token{
				Type:  hclsyntax.TokenQuotedLit,
				Bytes: []byte(line),
			}, nlWithIndent)
		}
	} else {
		tokens = append(tokens, &hclwrite.Token{
			Type:  hclsyntax.TokenQuotedLit,
			Bytes: []byte("\n" + text + "\n"),
		})
	}

	tokens = append(tokens, &hclwrite.Token{
		Type:  hclsyntax.TokenCHeredoc,
		Bytes: []byte(strings.TrimPrefix(heredocMarker, "-")),
	})

	return tokens
}
