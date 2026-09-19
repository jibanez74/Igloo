package helpers

import "database/sql"

// NullString returns an invalid sql.NullString for empty input.
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}

	return sql.NullString{String: s, Valid: true}
}

// NullInt64 returns an invalid sql.NullInt64 for 0.
func NullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{Valid: false}
	}

	return sql.NullInt64{Int64: i, Valid: true}
}

// NullFloat64 returns an invalid sql.NullFloat64 for 0.
func NullFloat64(f float64) sql.NullFloat64 {
	if f == 0 {
		return sql.NullFloat64{Valid: false}
	}

	return sql.NullFloat64{Float64: f, Valid: true}
}

// NullFloat64FromPtr returns an invalid sql.NullFloat64 for nil or 0.
func NullFloat64FromPtr(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}

	return NullFloat64(*f)
}

// StringPtrFromNull returns nil when the sql.NullString is invalid.
func StringPtrFromNull(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}

	return &s.String
}

// Float64PtrFromNull returns nil when the sql.NullFloat64 is invalid.
func Float64PtrFromNull(f sql.NullFloat64) *float64 {
	if !f.Valid {
		return nil
	}

	return &f.Float64
}
