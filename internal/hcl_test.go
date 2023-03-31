package internal_test

import (
	"bytes"

	"github.com/hashicorp/hcl/v2/hclwrite"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zclconf/go-cty/cty"

	"github.com/gideaworx/terraform-exporter-newrelic-plugin/internal"
)

const heredocText = `This
is
${a}
  test`

const indentedAndEscaped = `  This
  is
  $${a}
    test`

var _ = Describe("Hcl", func() {
	It("generates a heredoc without a dash", func() {
		test := hclwrite.NewFile()
		block := test.Body().AppendNewBlock("test", nil)
		block.Body().SetAttributeValue("numtest", cty.NumberIntVal(3))
		block.Body().SetAttributeValue("strtest", cty.StringVal("test"))
		block.Body().SetAttributeRaw("heredoctest", internal.CreateHeredoc(heredocText, "EOF", false))

		b := &bytes.Buffer{}
		b.WriteString("\n")
		_, err := test.WriteTo(b)
		Expect(err).NotTo(HaveOccurred())

		Expect(b.String()).To(ContainSubstring("heredoctest = <<EOF\n%s\nEOF", heredocText))
	})

	It("generates a heredoc with a dash", func() {
		test := hclwrite.NewFile()
		block := test.Body().AppendNewBlock("test", nil)
		block.Body().SetAttributeValue("numtest", cty.NumberIntVal(3))
		block.Body().SetAttributeValue("strtest", cty.StringVal("test"))
		block.Body().SetAttributeRaw("heredoctest", internal.CreateHeredoc(heredocText, "-EOF", true))

		b := &bytes.Buffer{}
		b.WriteString("\n")
		_, err := test.WriteTo(b)
		Expect(err).NotTo(HaveOccurred())
		Expect(b.String()).To(ContainSubstring("\n  heredoctest = <<-EOF\n%s\n  EOF", indentedAndEscaped))
	})
})
