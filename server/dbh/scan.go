package dbh

import "database/sql"

// ScanInt64Array takes the result of db.Query()
func ScanInt64Array(r *sql.Rows, queryErr error) ([]int64, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	defer r.Close()
	res := []int64{}
	for r.Next() {
		v := int64(0)
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
