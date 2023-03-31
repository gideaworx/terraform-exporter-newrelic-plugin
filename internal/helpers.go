package internal

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/mitchellh/pointerstructure"
	"github.com/zclconf/go-cty/cty"
)

// ToSnakeCase takes a string and performs the following transformations in this order:
// First Pass:
//   - Uppercase runes are converted to their lowercase counterparts.
//   - Lowercase and number runes are unmodified
//   - Any other printable runes are converted to _
//   - Non printable characters are discarded
//
// Second Pass:
//   - All leading and trailing _ are removed
//
// Third (Final) Pass
//   - All repeating _ are reduced to 1.
func ToSnakeCase(str string) string {
	firstPass := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsUpper(r):
			return unicode.ToLower(r)
		case unicode.IsLower(r), unicode.IsNumber(r):
			return r
		case unicode.IsPrint(r): // this is all the stuff that's not a number or letter that we want to convert to _
			return '_'
		default:
			return -1
		}
	}, str)

	secondPass := strings.TrimPrefix(strings.TrimSuffix(firstPass, "_"), "_")

	thirdPass := []string{}
	for _, chunk := range strings.Split(secondPass, "_") {
		if chunk != "" {
			thirdPass = append(thirdPass, chunk)
		}
	}

	return strings.Join(thirdPass, "_")
}

// IndexOf takes a slice and searches through it for a given value, returning
// the first index where it was found, or -1 if it was not found (or if haystack
// is empty or nil)
func IndexOf[T comparable](needle T, haystack []T) int {
	for i, s := range haystack {
		if s == needle {
			return i
		}
	}

	return -1
}

// IndexOf takes a slice of structs and searches it for a given value, where the field
// specified in the parameters matches the field of the struct, returning the first item
// or -1 if it was not found (or if haystack is empty or nil, or if the key does not
// exist on needle)
func IndexOfWithField[T any](needle T, haystack []T, fieldName string) int {
	needleVal, err := pointerstructure.Get(needle, fmt.Sprintf("/%s", fieldName))
	if err != nil {
		return -1
	}

	for i, s := range haystack {
		stalk, err := pointerstructure.Get(s, fmt.Sprintf("/%s", fieldName))
		if err != nil {
			continue
		}

		if reflect.DeepEqual(needleVal, stalk) {
			return i
		}
	}

	return -1
}

// ToCtyList will take a slice of strings and return a cty.ListVal. If the slice
// is empty or nil, this returns an empty cty.ListVal
func ToCtyList(vals []string) cty.Value {
	if len(vals) == 0 {
		return cty.ListValEmpty(cty.String)
	}

	valSlice := make([]cty.Value, len(vals))

	for i := range vals {
		valSlice[i] = cty.StringVal(vals[i])
	}

	return cty.ListVal(valSlice)
}
