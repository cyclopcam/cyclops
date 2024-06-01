package dbh

import "database/sql"

// ScanArray takes the result of db.Query() and returns an array of the given type.
// This is for queries that return a single column.
func ScanArray[T any](r *sql.Rows, queryErr error) ([]T, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer r.Close()
	res := []T{}
	for r.Next() {
		var v T
		if err := r.Scan(&v); err != nil {
			return nil, err
		}
		res = append(res, v)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return res, nil
}
