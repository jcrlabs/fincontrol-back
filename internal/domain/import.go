package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// ImportRow is a parsed row from a CSV or OFX file.
type ImportRow struct {
	Date        time.Time
	Description string
	Amount      decimal.Decimal
	Currency    string
	Hash        string // sha256(date+amount+description) — used for dedup
}

// ColumnMapping maps CSV column indices to ImportRow fields.
type ColumnMapping struct {
	DateCol         int    `json:"date_col"`
	DateFormat      string `json:"date_format"`      // e.g. "2006-01-02", "02/01/2006"
	DescriptionCol  int    `json:"description_col"`
	AmountCol       int    `json:"amount_col"`       // -1 if using DebitCol/CreditCol
	DebitCol        int    `json:"debit_col"`        // -1 if not present (use AmountCol)
	CreditCol       int    `json:"credit_col"`       // -1 if not present (use AmountCol)
	CurrencyCol     int    `json:"currency_col"`     // -1 = not present, use default
	DefaultCurrency string `json:"default_currency"` // e.g. "EUR"
	SkipRows        int    `json:"skip_rows"`        // header rows to skip
	Separator       string `json:"separator"`        // field separator: ",", ";", "\t"
}

// ImportPreview is returned by the preview endpoint.
type ImportPreview struct {
	Rows             []ImportRow   `json:"rows"`
	SuggestedMapping ColumnMapping `json:"suggested_mapping"`
	TotalRows        int           `json:"total_rows"`
}

// ImportResult is returned by the confirm endpoint.
type ImportResult struct {
	Imported   int      `json:"imported"`
	Duplicates int      `json:"duplicates"`
	Errors     []string `json:"errors"`
}
