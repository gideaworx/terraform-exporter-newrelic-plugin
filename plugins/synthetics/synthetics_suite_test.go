package synthetics_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSynthetics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Synthetics Suite")
}
