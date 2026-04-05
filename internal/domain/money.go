package domain

import "github.com/shopspring/decimal"

// Money is a value object combining an amount with a currency.
// RULE: NEVER use float64 for monetary amounts — always decimal.
type Money struct {
	Amount   decimal.Decimal
	Currency string // ISO 4217 (EUR, USD, GBP...)
}

func NewMoney(amount decimal.Decimal, currency string) Money {
	return Money{Amount: amount, Currency: currency}
}

func (m Money) IsZero() bool {
	return m.Amount.IsZero()
}

func (m Money) Add(other Money) Money {
	return Money{Amount: m.Amount.Add(other.Amount), Currency: m.Currency}
}

func (m Money) Neg() Money {
	return Money{Amount: m.Amount.Neg(), Currency: m.Currency}
}
