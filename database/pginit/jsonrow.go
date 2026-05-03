package pginit

import (
	"fmt"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
)

// JSONRowToAddrOfStruct is a generic [pgx.RowToFunc] that scans a single
// JSON-encoded column from row and unmarshals it into a freshly-allocated
// *T. Convenient for queries shaped like
//
//	SELECT json_agg(...) FROM (...)  // returns one JSON column.
//
// Callers using [pgx.CollectRows] / [pgx.CollectExactlyOneRow] can pass
// JSONRowToAddrOfStruct[MyType] as the row collector.
func JSONRowToAddrOfStruct[T any](row pgx.CollectableRow) (*T, error) {
	var dest T

	var jsonBytes []byte
	// scan row into []byte
	if pgxErr := row.Scan(&jsonBytes); pgxErr != nil {
		return nil, fmt.Errorf("could not scan row: %w", pgxErr)
	}

	// unmarshal []byte into struct
	if jsonErr := json.Unmarshal(jsonBytes, &dest); jsonErr != nil {
		return nil, fmt.Errorf("could not unmarshal json: %w", jsonErr)
	}

	return &dest, nil
}
