package internal_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zclconf/go-cty/cty"

	"github.com/gideaworx/terraform-exporter-newrelic-plugin/internal"
)

var _ = Describe("Helpers", func() {
	haystack := []string{"hi", "Hi", "", "a"}

	Describe("ToSnakeCase", func() {
		It("Lower cases and underscores spaces", func() {
			testCase := "Hello World"
			Expect(internal.ToSnakeCase(testCase)).To(Equal("hello_world"))
		})

		It("Replaces non-alphanums with underscores", func() {
			testCase := "Hello/With-Emoji🙂And!Numb3rs"
			Expect(internal.ToSnakeCase(testCase)).To(Equal("hello_with_emoji_and_numb3rs"))
		})

		It("Removes Leading Underscores", func() {
			testCase := " hello WORLD"
			Expect(internal.ToSnakeCase(testCase)).To(Equal("hello_world"))
		})

		It("Removes Trailing Underscores", func() {
			testCase := "Hello World🙂"
			Expect(internal.ToSnakeCase(testCase)).To(Equal("hello_world"))
		})

		It("Removes Repeating Underscores", func() {
			testCase := "Hello, world"
			Expect(internal.ToSnakeCase(testCase)).To(Equal("hello_world"))
		})

		It("Discards non-printable characters", func() {
			testCase := "hello\u00a0world"
			Expect(internal.ToSnakeCase(testCase)).To(Equal("helloworld"))
		})
	})

	Describe("IndexOf", func() {
		It("Returns a Non-Negative Index", func() {
			Expect(internal.IndexOf("hi", haystack)).To(Equal(0))
		})

		It("Returns -1 if not found", func() {
			Expect(internal.IndexOf("fhqwgads", haystack)).To(Equal(-1))
		})

		It("Returns -1 if haystack is empty or nil", func() {
			Expect(internal.IndexOf("hi", []string{})).To(Equal(-1))
			Expect(internal.IndexOf("hi", nil)).To(Equal(-1))
		})
	})

	Describe("ToCtyList", func() {
		It("Creates the list properly", func() {
			list := internal.ToCtyList(haystack)
			vals := list.AsValueSlice()

			Expect(len(haystack)).To(Equal(len(vals)))
			for i := range haystack {
				Expect(vals[i].Type()).To(Equal(cty.String))
				Expect(vals[i].AsString()).To(Equal(haystack[i]))
			}
		})

		It("Creates an empty list properly", func() {
			list := internal.ToCtyList([]string{})
			vals := list.AsValueSlice()

			Expect(len(vals)).To(Equal(list.LengthInt()))
			Expect(len(vals)).To(Equal(0))
		})

		It("Creates an nil list properly", func() {
			list := internal.ToCtyList(nil)
			vals := list.AsValueSlice()

			Expect(len(vals)).To(Equal(list.LengthInt()))
			Expect(len(vals)).To(Equal(0))
		})
	})
})
