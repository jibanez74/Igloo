package helpers

import (
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func GetPreciseDecimalFromStr(str string) (pgtype.Numeric, error) {
	d, err := decimal.NewFromString(str)
	if err != nil {
		return pgtype.Numeric{}, err
	}

	var n pgtype.Numeric

	err = n.Scan(d)
	if err != nil {
		return pgtype.Numeric{}, err
	}

	n.Valid = true

	return n, nil

}
