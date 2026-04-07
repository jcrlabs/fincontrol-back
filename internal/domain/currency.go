package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ExchangeRate stores the conversion rate between two currencies at a point in time.
type ExchangeRate struct {
	ID           uuid.UUID
	FromCurrency string
	ToCurrency   string
	Rate         decimal.Decimal
	Date         time.Time
}
