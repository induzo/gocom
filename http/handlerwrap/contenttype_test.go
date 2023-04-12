package handlerwrap

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

//nolint:gochecknoglobals // selection of pre-defined values used for testing
var (
	instEmpty        = MediaType{}
	instSimple       = NewMediaType("text/plain")
	instWildcard     = NewMediaType("*/*")
	instTextWildcard = NewMediaType("text/*")
	instParams       = NewMediaType("application/json; q=0.001; charset=utf-8")
	instJSON         = NewMediaType("application/json")
	instJSON2        = NewMediaType("application/json; charset=utf-8")
	instAppWildcard  = NewMediaType("application/*")
)

func TestNewMediaType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		value  string
		result MediaType
	}{
		{name: "Empty string", value: "", result: MediaType{}},
		{name: "Type and subtype", value: "application/json", result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}},
		{name: "Type, subtype, parameter", value: "a/b;c=d", result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d"}}},
		{name: "Subtype only", value: "/b", result: MediaType{}},
		{name: "Type only", value: "a/", result: MediaType{}},
		{name: "Type, subtype, invalid parameter", value: "a/b;c", result: MediaType{}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := NewMediaType(testCase.value)

			if result.Type != testCase.result.Type || result.Subtype != testCase.result.Subtype {
				t.Fatalf("Invalid content type, got %s/%s, exptected %s/%s for %s", result.Type, result.Subtype, testCase.result.Type, testCase.result.Subtype, testCase.value)
			} else if !reflect.DeepEqual(result.Parameters, testCase.result.Parameters) {
				t.Fatalf("Wrong parameters, got %v, expected %v for %s", result.Parameters, testCase.result.Parameters, testCase.value)
			}
		})
	}
}

func TestParseMediaType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		value  string
		result MediaType
	}{
		{name: "Type and subtype", value: "application/json", result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}},
		{name: "Type and subtype with whitespaces", value: "application/json   ", result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}},
		{name: "Type, subtype, parameter", value: "a/b;c=d", result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d"}}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseMediaType(testCase.value)
			if err != nil {
				t.Errorf("Expected an error for %s", testCase.value)
			} else if result.Type != testCase.result.Type || result.Subtype != testCase.result.Subtype {
				t.Fatalf("Invalid content type, got %s/%s, exptected %s/%s for %s", result.Type, result.Subtype, testCase.result.Type, testCase.result.Subtype, testCase.value)
			} else if !reflect.DeepEqual(result.Parameters, testCase.result.Parameters) {
				t.Fatalf("Wrong parameters, got %v, expected %v for %s", result.Parameters, testCase.result.Parameters, testCase.value)
			}
		})
	}
}

func TestParseMediaTypeErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		value string
		err   error
	}{
		{name: "Empty string", value: "", err: ErrInvalidMediaType},
		{name: "Subtype only", value: "/b", err: ErrInvalidMediaType},
		{name: "Type only", value: "a/", err: ErrInvalidMediaType},
		{name: "Type, subtype, invalid parameter", value: "a/b;c", err: ErrInvalidParameter},
		{name: "Type and parameter without subtype", value: "a/;c", err: ErrInvalidMediaType},
		{name: "Type and subtype with remaining data", value: "a/b c", err: ErrInvalidMediaType},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseMediaType(testCase.value)
			if err == nil {
				t.Errorf("Expected an error for %s", testCase.value)
			} else if !errors.Is(err, testCase.err) {
				t.Errorf("Unexpected error \"%v\", expected \"%v\" for %s", err, testCase.err, testCase.value)
			}
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		value  MediaType
		result string
	}{
		{name: "Empty media type", value: MediaType{}, result: ""},
		{name: "Type and subtype", value: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, result: "application/json"},
		{name: "Type, subtype, parameter", value: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d"}}, result: "a/b;c=d"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			result := testCase.value.String()

			if result != testCase.result {
				t.Errorf("Invalid result type, got %s, exptected %s", result, testCase.result)
			}
		})
	}
}

func TestMediaType_MIME(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		value  MediaType
		result string
	}{
		{name: "Empty media type", value: MediaType{}, result: ""},
		{name: "Type and subtype", value: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, result: "application/json"},
		{name: "Type, subtype, parameter", value: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d"}}, result: "a/b"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := testCase.value.MIME()

			if result != testCase.result {
				t.Errorf("Invalid result type, got %s, exptected %s", result, testCase.result)
			}
		})
	}
}

func TestMediaType_IsWildcard(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		value  MediaType
		result bool
	}{
		{name: "Empty media type", value: MediaType{}, result: false},
		{name: "Type and subtype", value: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, result: false},
		{name: "Type, subtype, parameter", value: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d"}}, result: false},
		{name: "text/*", value: MediaType{Type: "text", Subtype: "*"}, result: true},
		{name: "application/*; charset=utf-8", value: MediaType{Type: "application", Subtype: "*", Parameters: Parameters{"charset": "utf-8"}}, result: true},
		{name: "*/*", value: MediaType{Type: "*", Subtype: "*"}, result: true},
		// invalid MIME type, but will return true
		{name: "*/json", value: MediaType{Type: "*", Subtype: "json"}, result: true},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := testCase.value.IsWildcard()
			if result != testCase.result {
				t.Errorf("Invalid result type, got %v, expected %v", result, testCase.result)
			}
		})
	}
}

func TestGetMediaType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header string
		result MediaType
	}{
		{name: "Empty header", header: "", result: MediaType{}},
		{name: "Type and subtype", header: "application/json", result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}},
		{name: "Wildcard", header: "*/*", result: MediaType{Type: "*", Subtype: "*", Parameters: Parameters{}}},
		{name: "Capital subtype", header: "Application/JSON", result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}},
		{name: "Space in front of type", header: " application/json ", result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}},
		{name: "Capital and parameter", header: "Application/XML;charset=utf-8", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"charset": "utf-8"}}},
		{name: "Spaces around semicolon", header: "a/b ; c=d", result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d"}}},
		{name: "Spaces around semicolons", header: "a/b ; c=d ; e=f", result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d", "e": "f"}}},
		{name: "Two spaces around semicolons", header: "a/b  ;  c=d  ;  e=f", result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"c": "d", "e": "f"}}},
		{name: "White space after parameter", header: "application/xml;foo=bar ", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"foo": "bar"}}},
		{name: "White space after subtype and before parameter", header: "application/xml ; foo=bar ", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"foo": "bar"}}},
		{name: "Quoted parameter", header: "application/xml;foo=\"bar\" ", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"foo": "bar"}}},
		{name: "Quoted empty parameter", header: "application/xml;foo=\"\" ", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"foo": ""}}},
		{name: "Quoted pair", header: "application/xml;foo=\"\\\"b\" ", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"foo": "\"b"}}},
		{name: "Whitespace after quoted parameter", header: "application/xml;foo=\"\\\"B\" ", result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{"foo": "\"b"}}},
		{name: "Plus in subtype", header: "a/b+c;a=b;c=d", result: MediaType{Type: "a", Subtype: "b+c", Parameters: Parameters{"a": "b", "c": "d"}}},
		{name: "Capital parameter", header: "a/b;A=B", result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{"a": "b"}}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "http://test.test", nil)

			if len(testCase.header) > 0 {
				request.Header.Set("Content-Type", testCase.header)
			}

			result, err := GetMediaType(request)
			if err != nil {
				t.Errorf("Unexpected error \"%v\" for %s", err, testCase.header)
			} else if result.Type != testCase.result.Type || result.Subtype != testCase.result.Subtype {
				t.Errorf("Invalid content type, got %s/%s, exptected %s/%s for %s", result.Type, result.Subtype, testCase.result.Type, testCase.result.Subtype, testCase.header)
			} else if !reflect.DeepEqual(result.Parameters, testCase.result.Parameters) {
				t.Errorf("Wrong parameters, got %v, expected %v for %s", result.Parameters, testCase.result.Parameters, testCase.header)
			}
		})
	}
}

func TestGetMediaTypeErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header string
		err    error
	}{
		{"Type only", "Application", ErrInvalidMediaType},
		{"Subtype only", "/Application", ErrInvalidMediaType},
		{"Type with slash", "Application/", ErrInvalidMediaType},
		{"Invalid token character", "a/b\x19", ErrInvalidMediaType},
		{"Invalid character after subtype", "Application/JSON/test", ErrInvalidMediaType},
		{"No parameter name", "application/xml;=bar ", ErrInvalidParameter},
		{"Whitespace and no parameter name", "application/xml; =bar ", ErrInvalidParameter},
		{"No value and whitespace", "application/xml;foo= ", ErrInvalidParameter},
		{"Invalid character in value", "a/b;c=\x19", ErrInvalidParameter},
		{"Invalid character in quoted string", "a/b;c=\"\x19\"", ErrInvalidParameter},
		{"Invalid character in quoted pair", "a/b;c=\"\\\x19\"", ErrInvalidParameter},
		{"No assignment after parameter", "a/b;c", ErrInvalidParameter},
		{"No semicolon before parameter", "a/b e", ErrInvalidMediaType},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, "http://test.test", nil)

			if len(testCase.header) > 0 {
				request.Header.Set("Content-Type", testCase.header)
			}

			_, err := GetMediaType(request)
			if err == nil {
				t.Errorf("Expected an error for %s", testCase.header)
			} else if !errors.Is(err, testCase.err) {
				t.Errorf("Unexpected error \"%v\", expected \"%v\" for %s", err, testCase.err, testCase.header)
			}
		})
	}
}

func TestGetAcceptableMediaType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		header              string
		availableMediaTypes []MediaType
		result              MediaType
		extensionParameters Parameters
	}{
		{name: "Empty header", availableMediaTypes: []MediaType{
			{Type: "application", Subtype: "json", Parameters: Parameters{}},
		}, result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Type and subtype", header: "application/json", availableMediaTypes: []MediaType{
			{Type: "application", Subtype: "json", Parameters: Parameters{}},
		}, result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Capitalized type and subtype", header: "Application/Json", availableMediaTypes: []MediaType{
			{Type: "application", Subtype: "json", Parameters: Parameters{}},
		}, result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Multiple accept types", header: "text/plain,application/xml", availableMediaTypes: []MediaType{
			{Type: "text", Subtype: "plain", Parameters: Parameters{}},
		}, result: MediaType{Type: "text", Subtype: "plain", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Multiple accept types, second available", header: "text/plain,application/xml", availableMediaTypes: []MediaType{
			{Type: "application", Subtype: "xml", Parameters: Parameters{}},
		}, result: MediaType{Type: "application", Subtype: "xml", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Accept weight", header: "text/plain;q=1.0", availableMediaTypes: []MediaType{
			{Type: "text", Subtype: "plain", Parameters: Parameters{}},
		}, result: MediaType{Type: "text", Subtype: "plain", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Wildcard", header: "*/*", availableMediaTypes: []MediaType{
			{Type: "application", Subtype: "json", Parameters: Parameters{}},
		}, result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Wildcard subtype", header: "application/*", availableMediaTypes: []MediaType{
			{Type: "application", Subtype: "json", Parameters: Parameters{}},
		}, result: MediaType{Type: "application", Subtype: "json", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Weight with dot", header: "a/b;q=1.", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Multiple weights", header: "a/b;q=0.1,c/d;q=0.2", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
			{Type: "c", Subtype: "d", Parameters: Parameters{}},
		}, result: MediaType{Type: "c", Subtype: "d", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Multiple weights and default weight", header: "a/b;q=0.2,c/d;q=0.2", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
			{Type: "c", Subtype: "d", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Wildcard subtype and weight", header: "a/*;q=0.2,a/c", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
			{Type: "a", Subtype: "c", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "c", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Different accept order", header: "a/b,a/a", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Wildcard subtype with multiple available types", header: "a/*", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "a", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Wildcard subtype against weighted type", header: "a/a;q=0.2,a/*", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Media type parameter", header: "a/a;q=0.2,a/a;c=d", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
			{Type: "a", Subtype: "a", Parameters: Parameters{"c": "d"}},
		}, result: MediaType{Type: "a", Subtype: "a", Parameters: Parameters{"c": "d"}}, extensionParameters: Parameters{}},
		{name: "Weight and media type parameter", header: "a/b;q=1;e=e", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{"e": "e"}},
		{header: "a/*,a/a;q=0", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Maximum length weight", header: "a/a;q=0.001,a/b;q=0.002", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
			{Type: "a", Subtype: "b", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "b", Parameters: Parameters{}}, extensionParameters: Parameters{}},
		{name: "Spaces around comma", header: "a/a;q=0.1 , a/b , a/c", availableMediaTypes: []MediaType{
			{Type: "a", Subtype: "a", Parameters: Parameters{}},
		}, result: MediaType{Type: "a", Subtype: "a", Parameters: Parameters{}}, extensionParameters: Parameters{}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "http://test.test", nil)

			if len(testCase.header) > 0 {
				request.Header.Set("Accept", testCase.header)
			}

			result, extensionParameters, err := GetAcceptableMediaType(request, testCase.availableMediaTypes)

			if err != nil {
				t.Errorf("Unexpected error \"%v\" for %s", err, testCase.header)
			} else if result.Type != testCase.result.Type || result.Subtype != testCase.result.Subtype {
				t.Errorf("Invalid content type, got %s/%s, exptected %s/%s for %s", result.Type, result.Subtype, testCase.result.Type, testCase.result.Subtype, testCase.header)
			} else if !reflect.DeepEqual(result.Parameters, testCase.result.Parameters) {
				t.Errorf("Wrong parameters, got %v, expected %v for %s", result.Parameters, testCase.result.Parameters, testCase.header)
			} else if !reflect.DeepEqual(extensionParameters, testCase.extensionParameters) {
				t.Errorf("Wrong extension parameters, got %v, expected %v for %s", extensionParameters, testCase.extensionParameters, testCase.header)
			}
		})
	}
}

func TestGetAcceptableMediaTypeErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                string
		header              string
		availableMediaTypes []MediaType
		err                 error
	}{
		{"No available type", "", []MediaType{}, ErrNoAvailableTypeGiven},
		{"No acceptable type", "application/xml", []MediaType{{Type: "application", Subtype: "json", Parameters: Parameters{}}}, ErrNoAcceptableTypeFound},
		{"Invalid character after subtype", "application/xml/", []MediaType{{Type: "application", Subtype: "json", Parameters: Parameters{}}}, ErrInvalidMediaRange},
		{"Comma after subtype with no parameter", "application/xml,", []MediaType{{Type: "application", Subtype: "json", Parameters: Parameters{}}}, ErrInvalidMediaType},
		{"Subtype only", "/xml", []MediaType{{Type: "application", Subtype: "json", Parameters: Parameters{}}}, ErrInvalidMediaType},
		{"Type with comma and without subtype", "application/,", []MediaType{{Type: "application", Subtype: "json", Parameters: Parameters{}}}, ErrInvalidMediaType},
		{"Invalid character", "a/b c", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidMediaRange},
		{"No value for parameter", "a/b;c", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidParameter},
		{"Wildcard type only", "*/b", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidMediaType},
		{"Invalid character in weight", "a/b;q=a", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidWeight},
		{"Weight bigger than 1.0", "a/b;q=11", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidWeight},
		{"More than 3 digits after dot", "a/b;q=1.0000", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidWeight},
		{"Invalid character after dot", "a/b;q=1.a", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidWeight},
		{"Invalid digit after dot", "a/b;q=1.100", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidWeight},
		{"Weight with two dots", "a/b;q=0..1", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrInvalidWeight},
		{"Type with weight zero only", "a/b;q=0", []MediaType{{Type: "a", Subtype: "b", Parameters: Parameters{}}}, ErrNoAcceptableTypeFound},
		{"No value for extension parameter", "a/a;q=1;ext=", []MediaType{{Type: "a", Subtype: "a", Parameters: Parameters{}}}, ErrInvalidParameter},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodGet, "http://test.test", nil)

			if len(testCase.header) > 0 {
				request.Header.Set("Accept", testCase.header)
			}

			_, _, err := GetAcceptableMediaType(request, testCase.availableMediaTypes)
			if err == nil {
				t.Errorf("Expected an error for %s", testCase.header)
			} else if !errors.Is(err, testCase.err) {
				t.Errorf("Unexpected error \"%v\", expected \"%v\" for %s", err, testCase.err, testCase.header)
			}
		})
	}
}

func TestMediaType_Equal(t *testing.T) {
	t.Parallel()

	// create a map of items to turn into a permutation, these should all be
	// different
	mtut := map[string]MediaType{
		"empty":        instEmpty,
		"simple":       instSimple,
		"wildcard":     instWildcard,
		"textwildcard": instTextWildcard,
		"params":       instParams,
		"json":         instJSON,
		"json2":        instJSON2,
		"appwildcard":  instAppWildcard,
	}

	type test struct {
		name string
		a    MediaType
		b    MediaType
		want bool
	}

	tests := []test{}

	// create permutation
	for outerName, outerMt := range mtut {
		for innerName, innerMt := range mtut {
			tests = append(tests,
				test{
					fmt.Sprintf("%s vs %s", outerName, innerName),
					outerMt,
					innerMt,
					outerName == innerName,
				})
		}
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.a.Equal(tt.b); got != tt.want {
				t.Errorf("MediaType.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaType_EqualsMIME(t *testing.T) {
	t.Parallel()

	// create a map of items to turn into a permutation, these should all be
	// different
	mtut := map[string]MediaType{
		"empty":        instEmpty,
		"simple":       instSimple,
		"wildcard":     instWildcard,
		"textwildcard": instTextWildcard,
		"appwildcard":  instAppWildcard,
		"params":       instParams,
	}

	type test struct {
		name string
		a    MediaType
		b    MediaType
		want bool
	}

	tests := []test{
		// all of these are equal
		{"params vs json", instParams, instJSON, true},
		{"params vs json2", instParams, instJSON2, true},
		{"json vs params", instJSON, instParams, true},
		{"json2 vs params", instJSON2, instParams, true},
		{"json vs json", instJSON, instJSON, true},
		{"json vs json2", instJSON, instJSON2, true},
		{"json2 vs json", instJSON2, instJSON, true},
		{"json2 vs json2", instJSON2, instJSON2, true},
	}

	// create permutation of the remaining tests from the map which are only equal
	// to themselves
	for outerName, outerMt := range mtut {
		for innerName, innerMt := range mtut {
			tests = append(tests,
				test{
					fmt.Sprintf("%s vs %s", outerName, innerName),
					outerMt,
					innerMt,
					outerName == innerName,
				})
		}
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.a.EqualsMIME(tt.b); got != tt.want {
				t.Errorf("MediaType.EqualsMIME() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaType_Matches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    MediaType
		b    MediaType
		want bool
	}{
		{"empty matches empty", instEmpty, instEmpty, true},
		{"text/plain matches text/plain", instSimple, instSimple, true},
		{"text/* matches text/plain", instTextWildcard, instSimple, true},
		{"*/* matches text/plain", instWildcard, instSimple, true},
		{"text/plain matches text/*", instSimple, instTextWildcard, true},
		{"text/plain matches */*", instSimple, instWildcard, true},
		{"text/plain doesn't match application/*", instSimple, instAppWildcard, false},
		{"text/* doesn't match application/*", instTextWildcard, instAppWildcard, false},
		{"*/* matches application/*", instWildcard, instAppWildcard, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.a.Matches(tt.b); got != tt.want {
				t.Errorf("MediaType.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaType_MatchesAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    MediaType
		bs   []MediaType
		want bool
	}{
		{"vs no list", instEmpty, nil, false},
		{"vs empty list", instEmpty, []MediaType{}, false},
		{"empty vs matching single", instEmpty, []MediaType{instEmpty}, true},
		{"empty vs non-matching single", instEmpty, []MediaType{instJSON}, false},
		{"empty vs second match", instEmpty, []MediaType{instJSON, instEmpty}, true},
		{"specific vs wildcard only", instSimple, []MediaType{instTextWildcard}, true},
		{"specific vs second item wildcard", instSimple, []MediaType{instJSON, instTextWildcard}, true},
		{"wildcard vs anything", instWildcard, []MediaType{instJSON}, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.a.MatchesAny(tt.bs...); got != tt.want {
				t.Errorf("MediaType.MatchesAny() = %v, want %v", got, tt.want)
			}
		})
	}
}
