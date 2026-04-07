package parser

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// CSVParser implements app.ImportParser for CSV files.
type CSVParser struct{}

func NewCSVParser() *CSVParser { return &CSVParser{} }

// dateFormats lists all supported date layouts in preference order.
// Ambiguous formats (DD/MM vs MM/DD) are resolved by detectDateFormat.
var dateFormats = []string{
	"2006-01-02",
	"02/01/2006",
	"01/02/2006",
	"02-01-2006",
	"01-02-2006",
	"02.01.2006",
	"2006/01/02",
	"20060102",
	"2/1/2006",
	"1/2/2006",
	"02/01/06",
	"01/02/06",
}

func (p *CSVParser) Parse(r io.Reader, mapping domain.ColumnMapping) ([]domain.ImportRow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	data = stripBOM(data)

	sep := runeFromSeparator(mapping.Separator)
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sep
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	all, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	currency := mapping.DefaultCurrency
	if currency == "" {
		currency = "EUR"
	}
	dateFormat := mapping.DateFormat
	if dateFormat == "" {
		dateFormat = "2006-01-02"
	}

	var rows []domain.ImportRow
	for i, record := range all {
		if i < mapping.SkipRows {
			continue
		}

		maxCol := colMax(mapping.DateCol, mapping.DescriptionCol, mapping.AmountCol,
			mapping.DebitCol, mapping.CreditCol)
		if len(record) <= maxCol {
			continue
		}

		dateStr := strings.TrimSpace(record[mapping.DateCol])
		date, err := time.Parse(dateFormat, dateStr)
		if err != nil {
			continue // skip unparseable rows (metadata, totals, etc.)
		}

		amount, err := resolveAmount(record, mapping)
		if err != nil {
			continue
		}

		description := strings.TrimSpace(record[mapping.DescriptionCol])
		cur := currency
		if mapping.CurrencyCol >= 0 && mapping.CurrencyCol < len(record) {
			if c := strings.TrimSpace(record[mapping.CurrencyCol]); c != "" {
				cur = c
			}
		}

		rows = append(rows, domain.ImportRow{
			Date:        date,
			Description: description,
			Amount:      amount,
			Currency:    cur,
			Hash:        rowHash(date, amount, description),
		})
	}
	return rows, nil
}

// SuggestMapping inspects the file and guesses separator, column positions, and date format.
func (p *CSVParser) SuggestMapping(r io.Reader) (domain.ColumnMapping, []domain.ImportRow, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return domain.ColumnMapping{}, nil, fmt.Errorf("read: %w", err)
	}
	data = stripBOM(data)

	sep, sepChar := detectSeparator(data)

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = sep
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable number of fields (metadata rows differ from data rows)

	all, err := reader.ReadAll()
	if err != nil {
		return domain.ColumnMapping{}, nil, fmt.Errorf("read csv: %w", err)
	}
	if len(all) == 0 {
		return domain.ColumnMapping{}, nil, fmt.Errorf("empty file")
	}

	// Find header row — skip metadata rows that precede it.
	headerIdx := findHeaderRow(all)

	mapping := domain.ColumnMapping{
		SkipRows:        headerIdx + 1,
		DefaultCurrency: "EUR",
		DateFormat:      "2006-01-02",
		CurrencyCol:     -1,
		AmountCol:       -1,
		DebitCol:        -1,
		CreditCol:       -1,
		Separator:       sepChar,
	}

	header := all[headerIdx]
	for i, h := range header {
		lower := strings.ToLower(strings.TrimSpace(h))
		switch {
		case isDateHeader(lower):
			mapping.DateCol = i
		case isDescriptionHeader(lower):
			if mapping.DescriptionCol == 0 && i != 0 {
				mapping.DescriptionCol = i
			} else if mapping.DescriptionCol == 0 {
				mapping.DescriptionCol = i
			}
		case isAmountHeader(lower):
			mapping.AmountCol = i
		case isDebitHeader(lower):
			mapping.DebitCol = i
		case isCreditHeader(lower):
			mapping.CreditCol = i
		case isCurrencyHeader(lower):
			mapping.CurrencyCol = i
		}
	}

	// If both debit and credit found, disable single amount col.
	if mapping.DebitCol >= 0 && mapping.CreditCol >= 0 {
		mapping.AmountCol = -1
	}
	// If no amount/debit/credit detected, default to col 2.
	if mapping.AmountCol < 0 && (mapping.DebitCol < 0 || mapping.CreditCol < 0) {
		mapping.AmountCol = 2
	}

	// Detect date format from first data row.
	dataRows := all[mapping.SkipRows:]
	if len(dataRows) > 0 && mapping.DateCol < len(dataRows[0]) {
		mapping.DateFormat = detectDateFormat(strings.TrimSpace(dataRows[0][mapping.DateCol]), dataRows, mapping.DateCol)
	}

	// Parse preview rows.
	var rows []domain.ImportRow
	for _, record := range dataRows {
		maxCol := colMax(mapping.DateCol, mapping.DescriptionCol, mapping.AmountCol,
			mapping.DebitCol, mapping.CreditCol)
		if len(record) <= maxCol {
			continue
		}
		date, err := time.Parse(mapping.DateFormat, strings.TrimSpace(record[mapping.DateCol]))
		if err != nil {
			continue
		}
		amount, err := resolveAmount(record, mapping)
		if err != nil {
			continue
		}
		desc := strings.TrimSpace(record[mapping.DescriptionCol])
		rows = append(rows, domain.ImportRow{
			Date: date, Description: desc, Amount: amount,
			Currency: mapping.DefaultCurrency,
			Hash:     rowHash(date, amount, desc),
		})
	}
	return mapping, rows, nil
}

// resolveAmount extracts the amount from a record, handling both single-column
// and split debit/credit column layouts.
func resolveAmount(record []string, mapping domain.ColumnMapping) (decimal.Decimal, error) {
	if mapping.DebitCol >= 0 && mapping.CreditCol >= 0 {
		debitStr := ""
		creditStr := ""
		if mapping.DebitCol < len(record) {
			debitStr = strings.TrimSpace(record[mapping.DebitCol])
		}
		if mapping.CreditCol < len(record) {
			creditStr = strings.TrimSpace(record[mapping.CreditCol])
		}
		debit := parseEuropeanDecimal(debitStr)
		credit := parseEuropeanDecimal(creditStr)
		// Debit (expense/outflow) stored as negative, credit (income/inflow) as positive.
		return credit.Sub(debit), nil
	}

	if mapping.AmountCol < 0 || mapping.AmountCol >= len(record) {
		return decimal.Zero, fmt.Errorf("amount column out of range")
	}
	raw := strings.TrimSpace(record[mapping.AmountCol])
	return parseEuropeanDecimalStrict(raw)
}

// parseEuropeanDecimal parses an amount string that may use European formatting
// (period as thousands separator, comma as decimal separator) or standard format.
// Returns Zero for empty or unparseable strings.
func parseEuropeanDecimal(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	d, err := parseEuropeanDecimalStrict(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// parseEuropeanDecimalStrict returns an error for empty or unparseable strings.
func parseEuropeanDecimalStrict(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, fmt.Errorf("empty amount")
	}
	// Normalize Unicode minus (U+2212) and non-breaking hyphen (U+2011) to ASCII hyphen.
	s = strings.ReplaceAll(s, "\u2212", "-")
	s = strings.ReplaceAll(s, "\u2011", "-")

	// Detect format based on separators present.
	hasDot := strings.Contains(s, ".")
	hasComma := strings.Contains(s, ",")

	var normalized string
	switch {
	case hasDot && hasComma:
		// Both present: determine which is thousands vs decimal.
		lastDot := strings.LastIndex(s, ".")
		lastComma := strings.LastIndex(s, ",")
		if lastComma > lastDot {
			// European: 1.500,00 → thousands=dot, decimal=comma
			normalized = strings.ReplaceAll(s, ".", "")
			normalized = strings.ReplaceAll(normalized, ",", ".")
		} else {
			// US: 1,500.00 → thousands=comma, decimal=dot
			normalized = strings.ReplaceAll(s, ",", "")
		}
	case hasComma && !hasDot:
		// Comma only: treat as decimal separator (European)
		normalized = strings.ReplaceAll(s, ",", ".")
	default:
		// Dot only or neither: standard format
		normalized = s
	}

	return decimal.NewFromString(normalized)
}

// detectSeparator samples the first line to identify the field separator.
func detectSeparator(data []byte) (rune, string) {
	firstLine := string(bytes.SplitN(data, []byte("\n"), 2)[0])
	counts := map[rune]int{
		';':  strings.Count(firstLine, ";"),
		',':  strings.Count(firstLine, ","),
		'\t': strings.Count(firstLine, "\t"),
		'|':  strings.Count(firstLine, "|"),
	}
	best := ','
	for r, c := range counts {
		if c > counts[best] {
			best = r
		}
	}
	return best, string(best)
}

// findHeaderRow returns the index of the first row that looks like a CSV header.
// A real header row has multiple cells that ARE keywords (not "Key: value" metadata).
func findHeaderRow(rows [][]string) int {
	for i, row := range rows {
		matches := 0
		for _, cell := range row {
			lower := strings.ToLower(strings.TrimSpace(cell))
			// Skip metadata cells — they usually contain ":" separating key from value.
			if strings.Contains(lower, ":") {
				continue
			}
			if isDateHeader(lower) || isDescriptionHeader(lower) || isAmountHeader(lower) ||
				isDebitHeader(lower) || isCreditHeader(lower) {
				matches++
			}
		}
		// Require at least 2 header-like cells (real headers have date + description + amount).
		if matches >= 2 {
			return i
		}
		if i >= 9 {
			break
		}
	}
	return 0
}

// detectDateFormat tries to determine the date format by sampling data rows.
// Resolves ambiguity between DD/MM/YYYY and MM/DD/YYYY by looking for day > 12.
func detectDateFormat(sample string, rows [][]string, dateCol int) string {
	candidates := dateFormats

	// Filter to formats that parse the sample.
	var matching []string
	for _, f := range candidates {
		if _, err := time.Parse(f, sample); err == nil {
			matching = append(matching, f)
		}
	}
	if len(matching) == 0 {
		return "2006-01-02"
	}
	if len(matching) == 1 {
		return matching[0]
	}

	// Ambiguous: check if any row has a day component > 12 to distinguish DD/MM from MM/DD.
	for _, record := range rows {
		if dateCol >= len(record) {
			continue
		}
		s := strings.TrimSpace(record[dateCol])
		// Try DD/MM/YYYY first — if first part > 12 it must be day.
		parts := strings.FieldsFunc(s, func(r rune) bool {
			return r == '/' || r == '-' || r == '.'
		})
		if len(parts) >= 2 {
			first := atoi(parts[0])
			second := atoi(parts[1])
			if first > 12 {
				// Must be DD/MM/YYYY or DD-MM-YYYY or DD.MM.YYYY
				for _, f := range matching {
					if strings.HasPrefix(f, "02") {
						return f
					}
				}
			}
			if second > 12 {
				// Must be MM/DD/YYYY
				for _, f := range matching {
					if strings.HasPrefix(f, "01") {
						return f
					}
				}
			}
		}
	}

	// Default to first match (prefer ISO formats).
	return matching[0]
}

// ensureUTF8 detects the encoding from BOM or byte patterns and converts to UTF-8.
// Handles UTF-8 BOM, UTF-16 LE/BE, and ISO-8859-1/Windows-1252.
func ensureUTF8(data []byte) []byte {
	// UTF-16 LE BOM: FF FE
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		enc := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
		decoded, _, err := transform.Bytes(enc.NewDecoder(), data)
		if err == nil {
			return decoded
		}
	}
	// UTF-16 BE BOM: FE FF
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		enc := unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
		decoded, _, err := transform.Bytes(enc.NewDecoder(), data)
		if err == nil {
			return decoded
		}
	}
	// UTF-8 BOM: EF BB BF
	bom := []byte{0xEF, 0xBB, 0xBF}
	if bytes.HasPrefix(data, bom) {
		data = data[len(bom):]
	}
	// If valid UTF-8, return as-is.
	if utf8.Valid(data) {
		return data
	}
	// Assume ISO-8859-1 / Windows-1252.
	decoded, _, err := transform.Bytes(charmap.ISO8859_1.NewDecoder(), data)
	if err == nil {
		return decoded
	}
	return data
}

// stripBOM removes a UTF-8 BOM if present (kept for compatibility).
func stripBOM(data []byte) []byte {
	return ensureUTF8(data)
}

func runeFromSeparator(s string) rune {
	if s == "" {
		return ','
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return ','
	}
	return r
}

func rowHash(date time.Time, amount decimal.Decimal, description string) string {
	raw := fmt.Sprintf("%s|%s|%s", date.Format("2006-01-02"), amount.String(), description)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
}

func colMax(vals ...int) int {
	m := 0
	for _, v := range vals {
		if v > m {
			m = v
		}
	}
	return m
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Header keyword matchers.

func isDateHeader(s string) bool {
	keywords := []string{"date", "fecha", "datum", "data", "dt", "value date", "fecha valor", "f.valor", "f. valor"}
	return containsAny(s, keywords)
}

func isDescriptionHeader(s string) bool {
	keywords := []string{"desc", "concept", "narr", "memo", "detail", "texto", "referencia", "ref", "benef", "concepto", "operac"}
	return containsAny(s, keywords)
}

func isAmountHeader(s string) bool {
	keywords := []string{"amount", "importe", "value", "betrag", "montant", "monto", "movimiento", "impor"}
	return containsAny(s, keywords)
}

func isDebitHeader(s string) bool {
	keywords := []string{"cargo", "debit", "ausgabe", "debet", "charge", "salida", "retiro", "débito"}
	return containsAny(s, keywords)
}

func isCreditHeader(s string) bool {
	keywords := []string{"abono", "credit", "einnahme", "avoir", "ingreso", "entrada", "depósito", "haber"}
	return containsAny(s, keywords)
}

func isCurrencyHeader(s string) bool {
	keywords := []string{"curr", "moneda", "währung", "devise", "divisa"}
	return containsAny(s, keywords)
}

func containsAny(s string, keywords []string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
