package pginit

import (
	"fmt"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5"
)

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
