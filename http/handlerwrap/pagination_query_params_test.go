package handlerwrap

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestNewPaginationParams(t *testing.T) {
	t.Parallel()

	type args struct {
		val       string
		col       string
		direction string
		limit     int
	}

	tests := []struct {
		name string
		args args
		want *PaginationParams
	}{
		{
			name: "happy path",
			args: args{val: "test", col: "id", direction: "forward", limit: 10},
			want: &PaginationParams{CursorValue: "test", CursorColumn: "id", CursorDirection: "forward", Limit: 10},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := NewPaginationParams(tt.args.val, tt.args.col, tt.args.direction, tt.args.limit); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPaginationParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaginationParamError_Error(t *testing.T) {
	t.Parallel()

	type fields struct {
		StartingAfterValue string
		EndingBeforeValue  string
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "happy path",
			fields: fields{StartingAfterValue: "start", EndingBeforeValue: "end"},
			want:   "failed to parse query parameters starting_after: `start` and ending_before: `end`, should be mutually exclusive",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &PaginationParamError{
				StartingAfterValue: tt.fields.StartingAfterValue,
				EndingBeforeValue:  tt.fields.EndingBeforeValue,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("PaginationParamError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaginationParamError_ToErrorResponse(t *testing.T) {
	t.Parallel()

	type fields struct {
		StartingAfterValue string
		EndingBeforeValue  string
	}

	tests := []struct {
		name   string
		fields fields
		want   *ErrorResponse
	}{
		{
			name:   "happy path",
			fields: fields{StartingAfterValue: "start", EndingBeforeValue: "end"},
			want: &ErrorResponse{
				Err:          &PaginationParamError{StartingAfterValue: "start", EndingBeforeValue: "end"},
				StatusCode:   http.StatusBadRequest,
				Error:        "pagination_param_error",
				ErrorMessage: "failed to parse query parameters starting_after: `start` and ending_before: `end`, should be mutually exclusive",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &PaginationParamError{
				StartingAfterValue: tt.fields.StartingAfterValue,
				EndingBeforeValue:  tt.fields.EndingBeforeValue,
			}
			if got := e.ToErrorResponse(); !got.IsEqual(tt.want) {
				t.Errorf("PaginationParamError.ToErrorResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLimitError_Error(t *testing.T) {
	t.Parallel()

	type fields struct {
		Value    string
		MaxLimit int
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "happy path",
			fields: fields{Value: "101", MaxLimit: 100},
			want:   "failed to parse query param `limit`: `101` should be a valid int between 1 and 100",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &ParseLimitError{Value: tt.fields.Value, MaxLimit: tt.fields.MaxLimit}
			if got := e.Error(); got != tt.want {
				t.Errorf("ParseLimitError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLimitError_ToErrorResponse(t *testing.T) {
	t.Parallel()

	type fields struct {
		Value    string
		MaxLimit int
	}

	tests := []struct {
		name   string
		fields fields
		want   *ErrorResponse
	}{
		{
			name:   "happy path",
			fields: fields{Value: "101", MaxLimit: 100},
			want: &ErrorResponse{
				Err:          &ParseLimitError{Value: "101", MaxLimit: 100},
				StatusCode:   http.StatusBadRequest,
				Error:        "parse_limit_error",
				ErrorMessage: "failed to parse query param `limit`: `101` should be a valid int between 1 and 100",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := &ParseLimitError{Value: tt.fields.Value, MaxLimit: tt.fields.MaxLimit}
			if got := e.ToErrorResponse(); !got.IsEqual(tt.want) {
				t.Errorf("ParseLimitError.ToErrorResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePaginationQueryParams(t *testing.T) {
	t.Parallel()

	type args struct {
		urlValue         string
		paginationColumn string
		defaultLimit     int
		maxLimit         int
	}

	tests := []struct {
		name    string
		args    args
		want    *PaginationParams
		wantErr bool
	}{
		{
			name: "happy path - forward pagination",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456&limit=5",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			want: &PaginationParams{
				CursorValue:     "abc-123-def-456",
				CursorColumn:    "id",
				CursorDirection: ForwardPagination,
				Limit:           5,
			},
			wantErr: false,
		},
		{
			name: "happy path - backward pagination",
			args: args{
				urlValue:         "http://example.com/example?ending_before=abc-123-def-456&limit=5",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			want: &PaginationParams{
				CursorValue:     "abc-123-def-456",
				CursorColumn:    "id",
				CursorDirection: BackwardPagination,
				Limit:           5,
			},
			wantErr: false,
		},
		{
			name: "starting_after and ending_before both used",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456&ending_before=abc-123-def-456&limit=5",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			wantErr: true,
		},
		{
			name: "limit not provided",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			want: &PaginationParams{
				CursorValue:     "abc-123-def-456",
				CursorColumn:    "id",
				CursorDirection: ForwardPagination,
				Limit:           10,
			},
			wantErr: false,
		},
		{
			name: "limit not int",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456&limit=abc",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			wantErr: true,
		},
		{
			name: "limit negative value",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456&limit=-1",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			wantErr: true,
		},
		{
			name: "limit < 1",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456&limit=0",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			wantErr: true,
		},
		{
			name: "limit > max limit",
			args: args{
				urlValue:         "http://example.com/example?starting_after=abc-123-def-456&limit=101",
				paginationColumn: "id",
				defaultLimit:     10,
				maxLimit:         100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url, err := url.Parse(tt.args.urlValue)
			if err != nil {
				t.Fatalf("failed to parse url: %s", err)
			}
			got, errR := ParsePaginationQueryParams(url, tt.args.paginationColumn, tt.args.defaultLimit, tt.args.maxLimit)
			if (errR != nil) != tt.wantErr {
				t.Errorf("ParsePaginationQueryParams() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePaginationQueryParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkParsePaginationQueryParams(b *testing.B) {
	url, err := url.Parse("http://example.com/example?starting_after=abc-123-def-456&limit=5")
	if err != nil {
		b.Fatalf("failed to parse url: %s", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ParsePaginationQueryParams(url, "id", 10, 100)
	}
}
