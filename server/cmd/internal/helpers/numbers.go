package helpers

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func GetPreciseDecimalFromStr(str string) (pgtype.Numeric, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(str))
	if err != nil {
		return pgtype.Numeric{}, err
	}

	d = d.Round(3)

	var n pgtype.Numeric

	err = n.Scan(d.String())
	if err != nil {
		return pgtype.Numeric{}, err
	}

	n.Valid = true

	return n, nil
}
