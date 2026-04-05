package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// OFXParser implements basic OFX/QFX SGML parsing (pre-XML OFX 1.x).
type OFXParser struct{}

func NewOFXParser() *OFXParser { return &OFXParser{} }

func (p *OFXParser) Parse(r io.Reader) ([]domain.ImportRow, error) {
	scanner := bufio.NewScanner(r)
	var rows []domain.ImportRow

	var (
		inTx        bool
		dtPosted    string
		trnAmt      string
		memo        string
		name        string
		curDef      = "EUR"
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		tag, value, hasSep := parseOFXLine(line)

		switch {
		case tag == "CURDEF" && hasSep:
			curDef = value
		case tag == "STMTTRN":
			inTx = true
			dtPosted, trnAmt, memo, name = "", "", "", ""
		case tag == "/STMTTRN":
			if inTx && dtPosted != "" && trnAmt != "" {
				row, err := buildOFXRow(dtPosted, trnAmt, memo, name, curDef)
				if err == nil {
					rows = append(rows, row)
				}
			}
			inTx = false
		case inTx && tag == "DTPOSTED" && hasSep:
			dtPosted = value
		case inTx && tag == "TRNAMT" && hasSep:
			trnAmt = value
		case inTx && tag == "MEMO" && hasSep:
			memo = value
		case inTx && tag == "NAME" && hasSep:
			name = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ofx: %w", err)
	}
	return rows, nil
}

func parseOFXLine(line string) (tag, value string, hasSep bool) {
	if !strings.HasPrefix(line, "<") {
		return "", "", false
	}
	end := strings.Index(line, ">")
	if end < 0 {
		return "", "", false
	}
	tag = strings.ToUpper(line[1:end])
	value = strings.TrimSpace(line[end+1:])
	hasSep = value != ""
	return tag, value, hasSep
}

func buildOFXRow(dtPosted, trnAmt, memo, name, currency string) (domain.ImportRow, error) {
	// OFX date: YYYYMMDD or YYYYMMDDHHMMSS
	dateStr := dtPosted
	if len(dateStr) > 8 {
		dateStr = dateStr[:8]
	}
	date, err := time.Parse("20060102", dateStr)
	if err != nil {
		return domain.ImportRow{}, fmt.Errorf("parse date %q: %w", dtPosted, err)
	}

	amount, err := decimal.NewFromString(trnAmt)
	if err != nil {
		return domain.ImportRow{}, fmt.Errorf("parse amount %q: %w", trnAmt, err)
	}

	description := memo
	if description == "" {
		description = name
	}

	return domain.ImportRow{
		Date:        date,
		Description: description,
		Amount:      amount,
		Currency:    currency,
		Hash:        rowHash(date, amount, description),
	}, nil
}
