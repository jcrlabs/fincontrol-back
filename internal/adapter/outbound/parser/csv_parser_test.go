package parser

import (
	"strings"
	"testing"

	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// Abanca: semicolon separator, DD/MM/YYYY, comma decimal.
const abancaCSV = `Fecha;Concepto;Importe;Saldo
01/03/2026;TRANSFERENCIA RECIBIDA;1500,00;3200,00
02/03/2026;SUPERMERCADO MERCADONA;-45,80;3154,20
15/03/2026;RECIBO LUZ ENDESA;-120,50;3033,70
`

// ING Spain: semicolon, DD/MM/YYYY, cargo/abono split columns.
const ingCSV = `Fecha;Descripción;Cargo (EUR);Abono (EUR);Saldo (EUR)
03/03/2026;AMAZON MARKETPLACE;;49,99;2950,01
05/03/2026;NÓMINA EMPRESA;0,00;2500,00;5450,01
10/03/2026;CARREFOUR;55,30;;5394,71
`

// Standard ISO: comma separator, YYYY-MM-DD, dot decimal.
const isoCSV = `date,description,amount,currency
2026-03-01,SALARY,2500.00,EUR
2026-03-05,SUPERMARKET,-45.80,EUR
2026-03-10,RENT,-800.00,EUR
`

// European thousands: 1.500,00 format.
const europeanThousandsCSV = `Fecha;Concepto;Importe
01/01/2026;VENTA INMUEBLE;150.000,00
15/01/2026;HIPOTECA;-1.250,75
`

// UTF-8 BOM prefix.
var bomCSV = "\xEF\xBB\xBFFecha;Concepto;Importe\n01/03/2026;TEST BOM;100,00\n"

// US format with comma thousands.
const usCSV = `Date,Description,Amount
03/15/2026,PAYCHECK,"2,500.00"
03/20/2026,GROCERY STORE,-85.50
`

func TestSuggestMapping_Abanca(t *testing.T) {
	p := NewCSVParser()
	mapping, rows, err := p.SuggestMapping(strings.NewReader(abancaCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping.Separator != ";" {
		t.Errorf("separator: got %q, want %q", mapping.Separator, ";")
	}
	if mapping.DateFormat != "02/01/2006" {
		t.Errorf("date format: got %q, want %q", mapping.DateFormat, "02/01/2006")
	}
	if len(rows) != 3 {
		t.Errorf("rows: got %d, want 3", len(rows))
	}
	if !rows[0].Amount.Equal(dec("1500.00")) {
		t.Errorf("row[0] amount: got %s, want 1500.00", rows[0].Amount)
	}
	if !rows[1].Amount.Equal(dec("-45.80")) {
		t.Errorf("row[1] amount: got %s, want -45.80", rows[1].Amount)
	}
}

func TestSuggestMapping_ING_SplitColumns(t *testing.T) {
	p := NewCSVParser()
	mapping, rows, err := p.SuggestMapping(strings.NewReader(ingCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping.DebitCol < 0 || mapping.CreditCol < 0 {
		t.Fatalf("expected debit/credit cols detected, got debit=%d credit=%d", mapping.DebitCol, mapping.CreditCol)
	}
	if len(rows) != 3 {
		t.Errorf("rows: got %d, want 3", len(rows))
	}
	// Row 0: credit 49.99 (no debit) → amount = +49.99... wait
	// ING: Cargo=debit(expense), Abono=credit(income)
	// amount = credit - debit
	// Row 0: cargo="", abono=49.99 → amount=49.99
	if !rows[0].Amount.Equal(dec("49.99")) {
		t.Errorf("row[0] amount: got %s, want 49.99", rows[0].Amount)
	}
	// Row 1: cargo=0, abono=2500 → amount=2500
	if !rows[1].Amount.Equal(dec("2500.00")) {
		t.Errorf("row[1] amount: got %s, want 2500.00", rows[1].Amount)
	}
	// Row 2: cargo=55.30, abono="" → amount=-55.30
	if !rows[2].Amount.Equal(dec("-55.30")) {
		t.Errorf("row[2] amount: got %s, want -55.30", rows[2].Amount)
	}
}

func TestSuggestMapping_ISO(t *testing.T) {
	p := NewCSVParser()
	mapping, rows, err := p.SuggestMapping(strings.NewReader(isoCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapping.DateFormat != "2006-01-02" {
		t.Errorf("date format: got %q, want %q", mapping.DateFormat, "2006-01-02")
	}
	if len(rows) != 3 {
		t.Errorf("rows: got %d, want 3", len(rows))
	}
}

func TestSuggestMapping_EuropeanThousands(t *testing.T) {
	p := NewCSVParser()
	_, rows, err := p.SuggestMapping(strings.NewReader(europeanThousandsCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(rows))
	}
	if !rows[0].Amount.Equal(dec("150000.00")) {
		t.Errorf("row[0] amount: got %s, want 150000.00", rows[0].Amount)
	}
	if !rows[1].Amount.Equal(dec("-1250.75")) {
		t.Errorf("row[1] amount: got %s, want -1250.75", rows[1].Amount)
	}
}

func TestSuggestMapping_BOM(t *testing.T) {
	p := NewCSVParser()
	_, rows, err := p.SuggestMapping(strings.NewReader(bomCSV))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("rows: got %d, want 1", len(rows))
	}
}

func TestParse_WithMapping(t *testing.T) {
	p := NewCSVParser()
	mapping := domain.ColumnMapping{
		DateCol:         0,
		DateFormat:      "02/01/2006",
		DescriptionCol:  1,
		AmountCol:       2,
		DebitCol:        -1,
		CreditCol:       -1,
		CurrencyCol:     -1,
		DefaultCurrency: "EUR",
		SkipRows:        1,
		Separator:       ";",
	}
	rows, err := p.Parse(strings.NewReader(abancaCSV), mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("rows: got %d, want 3", len(rows))
	}
	if rows[0].Description != "TRANSFERENCIA RECIBIDA" {
		t.Errorf("description: got %q", rows[0].Description)
	}
}

func TestParseEuropeanDecimal(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"1500,00", "1500.00"},
		{"-45,80", "-45.80"},
		{"150.000,00", "150000.00"},
		{"-1.250,75", "-1250.75"},
		{"1,500.00", "1500.00"},
		{"2500.00", "2500.00"},
		{"-800.00", "-800.00"},
		{"0", "0"},
		{"", "0"},
	}
	for _, c := range cases {
		got := parseEuropeanDecimal(c.input)
		want := dec(c.want)
		if !got.Equal(want) {
			t.Errorf("parseEuropeanDecimal(%q) = %s, want %s", c.input, got, c.want)
		}
	}
}

func TestDetectSeparator(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"Fecha;Concepto;Importe;Saldo", ";"},
		{"date,description,amount", ","},
		{"date\tdescription\tamount", "\t"},
	}
	for _, c := range cases {
		_, got := detectSeparator([]byte(c.line))
		if got != c.want {
			t.Errorf("detectSeparator(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}

// dec is a test helper to create a decimal from a string.
func dec(s string) decimal.Decimal {
	d, _ := parseEuropeanDecimalStrict(s)
	return d
}
