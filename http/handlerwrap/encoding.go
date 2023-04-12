package handlerwrap

import (
	"net/http"
)

// Encoding is the media type used to render the returned content.
type Encoding string

const (
	ApplicationJSON Encoding = "application/json"
)

// ParseAcceptedEncoding is used to parse the Accept header from the request and match it to
// supported types to render the response with.
// The default content type if there are no matches is "application/json".
func ParseAcceptedEncoding(req *http.Request) Encoding {
	mtype, _, acceptErr := GetAcceptableMediaType(req, []MediaType{
		NewMediaType(string(ApplicationJSON)),
	})

	if acceptErr != nil {
		return ApplicationJSON
	}

	return Encoding(mtype.String())
}
