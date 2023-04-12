package handlerwrap

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
)

// https://github.com/elnormous/contenttype

var (
	// ErrInvalidMediaType is returned when the media type in the Content-Type or Accept header is syntactically invalid.
	ErrInvalidMediaType = errors.New("invalid media type")
	// ErrInvalidMediaRange is returned when the range of media types in the Content-Type or
	// Accept header is syntactically invalid.
	ErrInvalidMediaRange = errors.New("invalid media range")
	// ErrInvalidParameter is returned when the media type parameter in the Content-Type or
	// Accept header is syntactically invalid.
	ErrInvalidParameter = errors.New("invalid parameter")
	// ErrInvalidExtensionParameter is returned when the media type extension parameter in the
	// Content-Type or Accept header is syntactically invalid.
	ErrInvalidExtensionParameter = errors.New("invalid extension parameter")
	// ErrNoAcceptableTypeFound is returned when Accept header contains only media types that
	// are not in the acceptable media type list.
	ErrNoAcceptableTypeFound = errors.New("no acceptable type found")
	// ErrNoAvailableTypeGiven is returned when the acceptable media type list is empty.
	ErrNoAvailableTypeGiven = errors.New("no available type given")
	// ErrInvalidWeight is returned when the media type weight in Accept header is syntactically invalid.
	ErrInvalidWeight = errors.New("invalid weight")
)

// Parameters represents media type parameters as a key-value map.
type Parameters = map[string]string

// MediaType holds the type, subtype and parameters of a media type.
type MediaType struct {
	Type       string
	Subtype    string
	Parameters Parameters
}

func isWhitespaceChar(c byte) bool {
	// RFC 7230, 3.2.3. Whitespace
	return c == 0x09 || c == 0x20 // HTAB or SP
}

func isDigitChar(c byte) bool {
	// RFC 5234, Appendix B.1. Core Rules
	return c >= 0x30 && c <= 0x39
}

func isAlphaChar(c byte) bool {
	// RFC 5234, Appendix B.1. Core Rules
	return (c >= 0x41 && c <= 0x5A) || (c >= 0x61 && c <= 0x7A)
}

func isTokenChar(c byte) bool { //nolint:cyclop // check if token
	// RFC 7230, 3.2.6. Field Value Components
	return c == '!' || c == '#' || c == '$' || c == '%' || c == '&' || c == '\'' || c == '*' ||
		c == '+' || c == '-' || c == '.' || c == '^' || c == '_' || c == '`' || c == '|' || c == '~' ||
		isDigitChar(c) ||
		isAlphaChar(c)
}

func isVisibleChar(c byte) bool {
	// RFC 5234, Appendix B.1. Core Rules
	return c >= 0x21 && c <= 0x7E
}

func isObsoleteTextChar(c byte) bool {
	// RFC 7230, 3.2.6. Field Value Components
	return c >= 0x80 && c <= 0xFF
}

func isQuotedTextChar(char byte) bool {
	// RFC 7230, 3.2.6. Field Value Components
	return isWhitespaceChar(char) ||
		char == 0x21 ||
		(char >= 0x23 && char <= 0x5B) ||
		(char >= 0x5D && char <= 0x7E) ||
		isObsoleteTextChar(char)
}

func isQuotedPairChar(c byte) bool {
	// RFC 7230, 3.2.6. Field Value Components
	return isWhitespaceChar(c) ||
		isVisibleChar(c) ||
		isObsoleteTextChar(c)
}

func skipWhitespaces(s string) string {
	// RFC 7230, 3.2.3. Whitespace
	for i := 0; i < len(s); i++ {
		if !isWhitespaceChar(s[i]) {
			return s[i:]
		}
	}

	return ""
}

func consumeToken(inputStr string) (string, string, bool) { //nolint:gocritic // helper func
	// RFC 7230, 3.2.6. Field Value Components
	for i := 0; i < len(inputStr); i++ {
		if !isTokenChar(inputStr[i]) {
			return strings.ToLower(inputStr[:i]), inputStr[i:], i > 0
		}
	}

	return strings.ToLower(inputStr), "", len(inputStr) > 0
}

func consumeQuotedString(inputStr string) (string, string, bool) { //nolint:gocritic // helper func
	// RFC 7230, 3.2.6. Field Value Components
	var stringBuilder strings.Builder

	index := 0
	for ; index < len(inputStr); index++ {
		if inputStr[index] == '\\' { //nolint:gocritic // parse quoted string
			index++
			if len(inputStr) <= index || !isQuotedPairChar(inputStr[index]) {
				return "", inputStr, false
			}

			stringBuilder.WriteByte(inputStr[index])
		} else if isQuotedTextChar(inputStr[index]) {
			stringBuilder.WriteByte(inputStr[index])
		} else {
			break
		}
	}

	return strings.ToLower(stringBuilder.String()), inputStr[index:], true
}

func consumeType(inputStr string) (string, string, string, bool) { //nolint:gocritic // helper func
	// RFC 7231, 3.1.1.1. Media Type
	inputStr = skipWhitespaces(inputStr)

	var (
		mediaType    string
		mediaSubtype string
		consumed     bool
	)

	mediaType, inputStr, consumed = consumeToken(inputStr)

	if !consumed {
		return "", "", inputStr, false
	}

	if inputStr == "" || inputStr[0] != '/' {
		return "", "", inputStr, false
	}

	inputStr = inputStr[1:] // skip the slash

	mediaSubtype, inputStr, consumed = consumeToken(inputStr)
	if !consumed {
		return "", "", inputStr, false
	}

	if mediaType == "*" && mediaSubtype != "*" {
		return "", "", inputStr, false
	}

	inputStr = skipWhitespaces(inputStr)

	return mediaType, mediaSubtype, inputStr, true
}

func consumeParameter(inputStr string) (string, string, string, bool) { //nolint:gocritic // helper func
	// RFC 7231, 3.1.1.1. Media Type
	inputStr = skipWhitespaces(inputStr)

	var (
		consumed bool
		key      string
	)

	if key, inputStr, consumed = consumeToken(inputStr); !consumed {
		return "", "", inputStr, false
	}

	if inputStr == "" || inputStr[0] != '=' {
		return "", "", inputStr, false
	}

	inputStr = inputStr[1:] // skip the equal sign

	var value string

	if len(inputStr) > 0 && inputStr[0] == '"' {
		inputStr = inputStr[1:] // skip the opening quote

		if value, inputStr, consumed = consumeQuotedString(inputStr); !consumed {
			return "", "", inputStr, false
		}

		if inputStr == "" || inputStr[0] != '"' {
			return "", "", inputStr, false
		}

		inputStr = inputStr[1:] // skip the closing quote
	} else {
		if value, inputStr, consumed = consumeToken(inputStr); !consumed {
			return "", "", inputStr, false
		}
	}

	inputStr = skipWhitespaces(inputStr)

	return key, value, inputStr, true
}

func getWeight(inputStr string) (int, bool) { //nolint:cyclop // need to parse quality value
	// RFC 7231, 5.3.1. Quality Values
	result := 0
	multiplier := 1000

	if len(inputStr) > 5 { //nolint:gomnd // the string must not have more than three digits after the decimal point
		return 0, false
	}

	for idx := 0; idx < len(inputStr); idx++ {
		switch idx {
		case 0:
			// the first character must be 0 or 1
			if inputStr[idx] != '0' && inputStr[idx] != '1' {
				return 0, false
			}

			result = int(inputStr[idx]-'0') * multiplier
			multiplier /= 10
		case 1:
			// the second character must be a dot
			if inputStr[idx] != '.' {
				return 0, false
			}
		default:
			// the remaining characters must be digits and the value can not be greater than 1.000
			if (inputStr[0] == '1' && inputStr[idx] != '0') ||
				!isDigitChar(inputStr[idx]) {
				return 0, false
			}

			result += int(inputStr[idx]-'0') * multiplier
			multiplier /= 10
		}
	}

	return result, true
}

func compareMediaTypes(checkMediaType, mediaType MediaType) bool {
	// RFC 7231, 5.3.2. Accept
	if (checkMediaType.Type == "*" || checkMediaType.Type == mediaType.Type) &&
		(checkMediaType.Subtype == "*" || checkMediaType.Subtype == mediaType.Subtype) {
		for checkKey, checkValue := range checkMediaType.Parameters {
			if value, found := mediaType.Parameters[checkKey]; !found || value != checkValue {
				return false
			}
		}

		return true
	}

	return false
}

func getPrecedence(checkMediaType, mediaType MediaType) bool {
	// RFC 7231, 5.3.2. Accept
	if mediaType.Type == "" || mediaType.Subtype == "" { // not set
		return true
	}

	if (mediaType.Type == "*" && checkMediaType.Type != "*") ||
		(mediaType.Subtype == "*" && checkMediaType.Subtype != "*") ||
		(len(mediaType.Parameters) < len(checkMediaType.Parameters)) {
		return true
	}

	return false
}

// NewMediaType parses the string and returns an instance of MediaType struct.
func NewMediaType(s string) MediaType {
	mediaType, err := ParseMediaType(s)
	if err != nil {
		return MediaType{}
	}

	return mediaType
}

// Converts the MediaType to string.
func (mediaType *MediaType) String() string {
	var stringBuilder strings.Builder

	if len(mediaType.Type) > 0 || len(mediaType.Subtype) > 0 {
		stringBuilder.WriteString(mediaType.Type)
		stringBuilder.WriteByte('/')
		stringBuilder.WriteString(mediaType.Subtype)

		for key, value := range mediaType.Parameters {
			stringBuilder.WriteByte(';')
			stringBuilder.WriteString(key)
			stringBuilder.WriteByte('=')
			stringBuilder.WriteString(value)
		}
	}

	return stringBuilder.String()
}

// MIME returns the MIME type without any of the parameters
func (mediaType MediaType) MIME() string {
	var stringBuilder strings.Builder

	if len(mediaType.Type) > 0 || len(mediaType.Subtype) > 0 {
		stringBuilder.WriteString(mediaType.Type)
		stringBuilder.WriteByte('/')
		stringBuilder.WriteString(mediaType.Subtype)
	}

	return stringBuilder.String()
}

// Equal checks whether the provided MIME media type matches this one
// including all parameters
func (mediaType MediaType) Equal(mt MediaType) bool {
	return reflect.DeepEqual(mediaType, mt)
}

// EqualsMIME checks whether the base MIME types match
func (mediaType MediaType) EqualsMIME(mt MediaType) bool {
	return (mediaType.Type == mt.Type) && (mediaType.Subtype == mt.Subtype)
}

// Matches checks whether the MIME media types match handling wildcards in either
func (mediaType MediaType) Matches(mt MediaType) bool {
	t := mediaType.Type == mt.Type || (mediaType.Type == "*") || (mt.Type == "*")
	st := mediaType.Subtype == mt.Subtype || mediaType.Subtype == "*" || mt.Subtype == "*"

	return t && st
}

// MatchesAny checks whether the MIME media types matches any of the specified
// list of mediatype handling wildcards in any of them
func (mediaType MediaType) MatchesAny(mts ...MediaType) bool {
	for _, mt := range mts {
		if mediaType.Matches(mt) {
			return true
		}
	}

	return false
}

// IsWildcard returns true if either the Type or Subtype are the wildcard character '*'
func (mediaType MediaType) IsWildcard() bool {
	return mediaType.Type == `*` || mediaType.Subtype == `*`
}

// GetMediaType gets the content of Content-Type header, parses it, and returns the parsed MediaType.
// If the request does not contain the Content-Type header, an empty MediaType is returned.
func GetMediaType(request *http.Request) (MediaType, error) {
	// RFC 7231, 3.1.1.5. Content-Type
	contentTypeHeaders := request.Header.Values("Content-Type")
	if len(contentTypeHeaders) == 0 {
		return MediaType{}, nil
	}

	return ParseMediaType(contentTypeHeaders[0])
}

// ParseMediaType parses the given string as a MIME media type (with optional parameters) and returns it as a MediaType.
// If the string cannot be parsed an appropriate error is returned.
func ParseMediaType(inputStr string) (MediaType, error) {
	// RFC 7231, 3.1.1.1. Media Type
	mediaType := MediaType{
		Parameters: Parameters{},
	}

	var consumed bool

	if mediaType.Type, mediaType.Subtype, inputStr, consumed = consumeType(inputStr); !consumed {
		return MediaType{}, ErrInvalidMediaType
	}

	for len(inputStr) > 0 && inputStr[0] == ';' {
		inputStr = inputStr[1:] // skip the semicolon

		key, value, remaining, consumed := consumeParameter(inputStr)
		if !consumed {
			return MediaType{}, ErrInvalidParameter
		}

		inputStr = remaining

		mediaType.Parameters[key] = value
	}

	// there must not be anything left after parsing the header
	if len(inputStr) > 0 {
		return MediaType{}, ErrInvalidMediaType
	}

	return mediaType, nil
}

// GetAcceptableMediaType chooses a media type from available media types according to the Accept.
// Returns the most suitable media type or an error if no type can be selected.
func GetAcceptableMediaType(request *http.Request, availableMediaTypes []MediaType) (MediaType, Parameters, error) {
	// RFC 7231, 5.3.2. Accept
	if len(availableMediaTypes) == 0 {
		return MediaType{}, Parameters{}, ErrNoAvailableTypeGiven
	}

	acceptHeaders := request.Header.Values("Accept")
	if len(acceptHeaders) == 0 {
		return availableMediaTypes[0], Parameters{}, nil
	}

	return GetAcceptableMediaTypeFromHeader(acceptHeaders[0], availableMediaTypes)
}

// GetAcceptableMediaTypeFromHeader chooses a media type from available media types
// according to the specified Accept header value.
// Returns the most suitable media type or an error if no type can be selected.
func GetAcceptableMediaTypeFromHeader( //nolint:gocognit,cyclop // parser
	headerValue string,
	availableMediaTypes []MediaType,
) (MediaType, Parameters, error) {
	headerStr := headerValue

	weights := make([]struct {
		mediaType           MediaType
		extensionParameters Parameters
		weight              int
		order               int
	}, len(availableMediaTypes))

	for mediaTypeCount := 0; len(headerStr) > 0; mediaTypeCount++ {
		if mediaTypeCount > 0 {
			// every media type after the first one must start with a comma
			if headerStr[0] != ',' {
				break
			}

			headerStr = headerStr[1:] // skip the comma
		}

		acceptableMediaType := MediaType{
			Parameters: Parameters{},
		}

		var consumed bool

		if acceptableMediaType.Type, acceptableMediaType.Subtype, headerStr, consumed = consumeType(headerStr); !consumed {
			return MediaType{}, Parameters{}, ErrInvalidMediaType
		}

		weight := 1000 // 1.000

		// media type parameters
		for len(headerStr) > 0 && headerStr[0] == ';' {
			headerStr = headerStr[1:] // skip the semicolon

			var key, value string

			if key, value, headerStr, consumed = consumeParameter(headerStr); !consumed {
				return MediaType{}, Parameters{}, ErrInvalidParameter
			}

			if key == "q" {
				if weight, consumed = getWeight(value); !consumed {
					return MediaType{}, Parameters{}, ErrInvalidWeight
				}

				break // "q" parameter separates media type parameters from Accept extension parameters
			}

			acceptableMediaType.Parameters[key] = value
		}

		extensionParameters := Parameters{}

		for len(headerStr) > 0 && headerStr[0] == ';' {
			headerStr = headerStr[1:] // skip the semicolon

			var key, value, remaining string

			if key, value, remaining, consumed = consumeParameter(headerStr); !consumed {
				return MediaType{}, Parameters{}, ErrInvalidParameter
			}

			headerStr = remaining

			extensionParameters[key] = value
		}

		for idx, availableMediaType := range availableMediaTypes {
			if compareMediaTypes(acceptableMediaType, availableMediaType) &&
				getPrecedence(acceptableMediaType, weights[idx].mediaType) {
				weights[idx].mediaType = acceptableMediaType
				weights[idx].extensionParameters = extensionParameters
				weights[idx].weight = weight
				weights[idx].order = mediaTypeCount
			}
		}

		headerStr = skipWhitespaces(headerStr)
	}

	// there must not be anything left after parsing the header
	if len(headerStr) > 0 {
		return MediaType{}, Parameters{}, ErrInvalidMediaRange
	}

	resultIndex := -1

	for idx, weight := range weights {
		if resultIndex != -1 {
			if weight.weight > weights[resultIndex].weight ||
				(weight.weight == weights[resultIndex].weight && weight.order < weights[resultIndex].order) {
				resultIndex = idx
			}
		} else if weight.weight > 0 {
			resultIndex = idx
		}
	}

	if resultIndex == -1 {
		return MediaType{}, Parameters{}, ErrNoAcceptableTypeFound
	}

	return availableMediaTypes[resultIndex], weights[resultIndex].extensionParameters, nil
}
