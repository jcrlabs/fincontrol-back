package parser

import (
	"strings"
	"testing"

	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal { d, _ := parseEuropeanDecimalStrict(s); return d }

func assertRows(t *testing.T, name string, rows []domain.ImportRow, want int) {
	t.Helper()
	if len(rows) != want {
		t.Errorf("%s: got %d rows, want %d", name, len(rows), want)
	}
}

func assertAmount(t *testing.T, name string, rows []domain.ImportRow, idx int, want string) {
	t.Helper()
	if idx >= len(rows) {
		t.Errorf("%s: row %d out of range (%d rows)", name, idx, len(rows))
		return
	}
	if !rows[idx].Amount.Equal(dec(want)) {
		t.Errorf("%s row[%d]: got %s, want %s", name, idx, rows[idx].Amount, want)
	}
}

const abancaSimple = "Fecha;Concepto;Importe;Saldo\n01/03/2026;TRANSFERENCIA;1500,00;3200,00\n02/03/2026;MERCADONA;-45,80;3154,20\n15/03/2026;LUZ ENDESA;-120,50;3033,70\n"
const abancaTwoDateCols = "Fecha Operación;Fecha Valor;Concepto;Importe;Divisa;Saldo\n01/03/2026;01/03/2026;TRANSFERENCIA;1500,00;EUR;3200,00\n02/03/2026;03/03/2026;PAGO TARJETA;-45,80;EUR;3154,20\n"
const abancaWithMetadata = "Cuenta:;ES12 3456 7890 12 3456789012\nTitular:;Jonathan\nPeriodo:;01/03/2026 - 31/03/2026\nMoneda:;EUR\n\nFecha;Concepto;Importe;Saldo\n01/03/2026;TRANSFERENCIA;1500,00;3200,00\n02/03/2026;MERCADONA;-45,80;3154,20\n"
const abancaMetadataFecha = "Nombre de la cuenta:;Cuenta Corriente\nIBAN:;ES12 3456 7890\nFecha inicio:;01/03/2026\nFecha fin:;31/03/2026\n\nFecha;Concepto;Importe;Saldo\n01/03/2026;NOMINA;2500,00;5000,00\n15/03/2026;HIPOTECA;-800,00;4200,00\n"
const abancaQuoted = "\"Fecha\";\"Concepto\";\"Importe\";\"Saldo\"\n\"01/03/2026\";\"TRANSFERENCIA\";\"1.500,00\";\"3.200,00\"\n\"02/03/2026\";\"MERCADONA\";\"-45,80\";\"3.154,20\"\n"
const abancaThousands = "Fecha;Concepto;Importe;Saldo\n01/01/2026;VENTA INMUEBLE;150.000,00;155.000,00\n15/01/2026;HIPOTECA;-1.250,75;153.749,25\n"
const abancaPositiveSign = "Fecha;Concepto;Importe;Saldo\n01/03/2026;NOMINA;+2.500,00;5.000,00\n05/03/2026;RECIBO GAS;-85,30;4.914,70\n"
const abancaEarlyMonth = "Fecha;Concepto;Importe;Saldo\n01/03/2026;PAGO A;100,00;1100,00\n02/03/2026;PAGO B;-50,00;1050,00\n03/03/2026;PAGO C;-25,00;1025,00\n"
const abancaEmptyRows = "Fecha;Concepto;Importe;Saldo\n01/03/2026;TRANSFERENCIA;1500,00;3200,00\n\n02/03/2026;MERCADONA;-45,80;3154,20\n\n"
const ingSplitCols = "Fecha;Descripción;Cargo (EUR);Abono (EUR);Saldo (EUR)\n03/03/2026;AMAZON;;49,99;2950,01\n05/03/2026;NOMINA;0,00;2500,00;5450,01\n10/03/2026;CARREFOUR;55,30;;5394,71\n"
const isoFormat = "date,description,amount,currency\n2026-03-01,SALARY,2500.00,EUR\n2026-03-05,SUPERMARKET,-45.80,EUR\n"
const tabSeparated = "Fecha\tConcepto\tImporte\tSaldo\n01/03/2026\tPAGO\t100,00\t1000,00\n"

var abancaUnicodeMinus = "Fecha;Concepto;Importe;Saldo\n01/03/2026;NOMINA;2500,00;5000,00\n05/03/2026;PAGO TARJETA;\u2212120,50;4879,50\n"
var bomFormat = "\xEF\xBB\xBFFecha;Concepto;Importe;Saldo\n01/03/2026;PAGO;100,00;1000,00\n"

func TestAbanca_Simple(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaSimple))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "simple", rows, 3)
	assertAmount(t, "simple", rows, 0, "1500")
	assertAmount(t, "simple", rows, 1, "-45.80")
}

func TestAbanca_TwoDateCols(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaTwoDateCols))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "two-date-cols", rows, 2)
	assertAmount(t, "two-date-cols", rows, 1, "-45.80")
}

func TestAbanca_MetadataRows(t *testing.T) {
	m, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaWithMetadata))
	if err != nil {
		t.Fatal(err)
	}
	if m.SkipRows < 5 {
		t.Errorf("SkipRows=%d, want >=5", m.SkipRows)
	}
	assertRows(t, "metadata", rows, 2)
}

func TestAbanca_MetadataWithFecha(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaMetadataFecha))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "metadata-fecha", rows, 2)
}

func TestAbanca_QuotedFields(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaQuoted))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "quoted", rows, 2)
	assertAmount(t, "quoted", rows, 0, "1500")
	assertAmount(t, "quoted", rows, 1, "-45.80")
}

func TestAbanca_UnicodeMinus(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaUnicodeMinus))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "unicode-minus", rows, 2)
	assertAmount(t, "unicode-minus", rows, 1, "-120.50")
}

func TestAbanca_Thousands(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaThousands))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "thousands", rows, 2)
	assertAmount(t, "thousands", rows, 0, "150000")
	assertAmount(t, "thousands", rows, 1, "-1250.75")
}

func TestAbanca_PositiveSign(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaPositiveSign))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "positive-sign", rows, 2)
	assertAmount(t, "positive-sign", rows, 0, "2500")
	assertAmount(t, "positive-sign", rows, 1, "-85.30")
}

func TestAbanca_EarlyMonth(t *testing.T) {
	m, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaEarlyMonth))
	if err != nil {
		t.Fatal(err)
	}
	if m.DateFormat != "02/01/2006" {
		t.Errorf("dateFormat=%q, want 02/01/2006", m.DateFormat)
	}
	assertRows(t, "early-month", rows, 3)
}

func TestAbanca_EmptyRows(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(abancaEmptyRows))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "empty-rows", rows, 2)
}

func TestING_SplitCols(t *testing.T) {
	m, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(ingSplitCols))
	if err != nil {
		t.Fatal(err)
	}
	if m.DebitCol < 0 || m.CreditCol < 0 {
		t.Fatalf("debit/credit not detected: d=%d c=%d", m.DebitCol, m.CreditCol)
	}
	assertRows(t, "ing-split", rows, 3)
	assertAmount(t, "ing-split", rows, 0, "49.99")
	assertAmount(t, "ing-split", rows, 1, "2500")
	assertAmount(t, "ing-split", rows, 2, "-55.30")
}

func TestISO_Format(t *testing.T) {
	m, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(isoFormat))
	if err != nil {
		t.Fatal(err)
	}
	if m.DateFormat != "2006-01-02" {
		t.Errorf("dateFormat=%q, want 2006-01-02", m.DateFormat)
	}
	assertRows(t, "iso", rows, 2)
}

func TestBOM_Format(t *testing.T) {
	_, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(bomFormat))
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, "bom", rows, 1)
}

func TestTab_Separated(t *testing.T) {
	m, rows, err := NewCSVParser().SuggestMapping(strings.NewReader(tabSeparated))
	if err != nil {
		t.Fatal(err)
	}
	if m.Separator != "\t" {
		t.Errorf("separator=%q, want tab", m.Separator)
	}
	assertRows(t, "tab", rows, 1)
}

func TestParseEuropeanDecimal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1500,00", "1500"}, {"-45,80", "-45.8"}, {"150.000,00", "150000"},
		{"-1.250,75", "-1250.75"}, {"1,500.00", "1500"}, {"2500.00", "2500"},
		{"-800.00", "-800"}, {"+2500,00", "2500"}, {"\u221245,80", "-45.8"},
		{"0", "0"}, {"", "0"},
	}
	for _, c := range cases {
		got := parseEuropeanDecimal(c.in)
		if !got.Equal(dec(c.want)) {
			t.Errorf("parseEuropeanDecimal(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestDetectSeparator(t *testing.T) {
	cases := []struct{ line, want string }{
		{"Fecha;Concepto;Importe;Saldo", ";"}, {"date,desc,amount", ","}, {"a\tb\tc", "\t"},
	}
	for _, c := range cases {
		_, got := detectSeparator([]byte(c.line))
		if got != c.want {
			t.Errorf("sep(%q)=%q, want %q", c.line, got, c.want)
		}
	}
}
