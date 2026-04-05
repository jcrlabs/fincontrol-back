package parser

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// CSVParser implements app.ImportParser for CSV files.
type CSVParser struct{}

func NewCSVParser() *CSVParser { return &CSVParser{} }

func (p *CSVParser) Parse(r io.Reader, mapping domain.ColumnMapping) ([]domain.ImportRow, error) {
	reader := csv.NewReader(r)
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
		if len(record) <= max(mapping.DateCol, mapping.DescriptionCol, mapping.AmountCol) {
			continue
		}

		date, err := time.Parse(dateFormat, strings.TrimSpace(record[mapping.DateCol]))
		if err != nil {
			continue // skip unparseable rows
		}

		amountStr := strings.ReplaceAll(strings.TrimSpace(record[mapping.AmountCol]), ",", ".")
		amount, err := decimal.NewFromString(amountStr)
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

// SuggestMapping inspects the header row and guesses column positions.
func (p *CSVParser) SuggestMapping(r io.Reader) (domain.ColumnMapping, []domain.ImportRow, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	all, err := reader.ReadAll()
	if err != nil {
		return domain.ColumnMapping{}, nil, fmt.Errorf("read csv: %w", err)
	}
	if len(all) == 0 {
		return domain.ColumnMapping{}, nil, fmt.Errorf("empty file")
	}

	mapping := domain.ColumnMapping{
		SkipRows:        1,
		DefaultCurrency: "EUR",
		DateFormat:      "2006-01-02",
		CurrencyCol:     -1,
	}

	// Heuristic: look for date/description/amount keywords in header
	header := all[0]
	for i, h := range header {
		lower := strings.ToLower(strings.TrimSpace(h))
		switch {
		case strings.Contains(lower, "date") || strings.Contains(lower, "fecha"):
			mapping.DateCol = i
		case strings.Contains(lower, "desc") || strings.Contains(lower, "concept") || strings.Contains(lower, "narr"):
			mapping.DescriptionCol = i
		case strings.Contains(lower, "amount") || strings.Contains(lower, "importe") || strings.Contains(lower, "value"):
			mapping.AmountCol = i
		case strings.Contains(lower, "curr") || strings.Contains(lower, "moneda"):
			mapping.CurrencyCol = i
		}
	}

	// Detect date format from first data row
	if len(all) > 1 && mapping.DateCol < len(all[1]) {
		mapping.DateFormat = detectDateFormat(strings.TrimSpace(all[1][mapping.DateCol]))
	}

	// Parse rows with suggested mapping
	var rows []domain.ImportRow
	for _, record := range all[mapping.SkipRows:] {
		if len(record) <= max(mapping.DateCol, mapping.DescriptionCol, mapping.AmountCol) {
			continue
		}
		date, err := time.Parse(mapping.DateFormat, strings.TrimSpace(record[mapping.DateCol]))
		if err != nil {
			continue
		}
		amountStr := strings.ReplaceAll(strings.TrimSpace(record[mapping.AmountCol]), ",", ".")
		amount, err := decimal.NewFromString(amountStr)
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

func detectDateFormat(s string) string {
	formats := []string{"2006-01-02", "02/01/2006", "01/02/2006", "02-01-2006", "20060102"}
	for _, f := range formats {
		if _, err := time.Parse(f, s); err == nil {
			return f
		}
	}
	return "2006-01-02"
}

func rowHash(date time.Time, amount decimal.Decimal, description string) string {
	raw := fmt.Sprintf("%s|%s|%s", date.Format("2006-01-02"), amount.String(), description)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
}

func max(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
