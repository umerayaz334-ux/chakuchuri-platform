package ratesheets

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Row struct {
	Courier string
	Service string
	Zone    string
	Weight  string
	Price   int
}

var ErrNoRates = errors.New("no rate rows found")

func Parse(raw []byte, filename string, defaultCourier string, defaultService string) ([]Row, error) {
	var rows [][]string
	var err error
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".xlsx":
		rows, err = readXLSX(raw)
	default:
		rows, err = readCSV(raw)
	}
	if err != nil {
		return nil, err
	}

	parsed := parseTable(rows, defaultCourier, defaultService)
	if len(parsed) == 0 {
		return nil, ErrNoRates
	}
	return parsed, nil
}

func readCSV(raw []byte) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	var rows [][]string
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv: %w", err)
		}
		rows = append(rows, cleanRow(row))
	}
	return rows, nil
}

func parseTable(rows [][]string, defaultCourier string, defaultService string) []Row {
	for index, row := range rows {
		headers := normalizedHeaders(row)
		if hasAny(headers, "price", "rate", "amount", "charge") && hasAny(headers, "zone", "weight", "weightband") {
			return parseTabular(headers, rows[index+1:], defaultCourier, defaultService)
		}
		if zoneHeaderCount(headers) >= 2 {
			return parseMatrix(row, rows[index+1:], defaultCourier, defaultService)
		}
	}
	return nil
}

func parseTabular(headers []string, rows [][]string, defaultCourier string, defaultService string) []Row {
	courierIndex := firstHeader(headers, "courier", "carrier")
	serviceIndex := firstHeader(headers, "service", "servicelevel", "product")
	zoneIndex := firstHeader(headers, "zone")
	weightIndex := firstHeader(headers, "weight", "weightband", "slab", "kg", "lbs")
	priceIndex := firstHeader(headers, "price", "rate", "amount", "charge")

	var parsed []Row
	for _, row := range rows {
		price := priceFromString(cell(row, priceIndex))
		zone := zoneFromString(cell(row, zoneIndex))
		weight := strings.TrimSpace(cell(row, weightIndex))
		if price < 1 || zone == "" || weight == "" {
			continue
		}
		parsed = append(parsed, Row{
			Courier: firstNonEmpty(cell(row, courierIndex), defaultCourier, "Custom courier"),
			Service: firstNonEmpty(cell(row, serviceIndex), defaultService, "Duty paid premium"),
			Zone:    zone,
			Weight:  weight,
			Price:   price,
		})
	}
	return parsed
}

func parseMatrix(headerRow []string, rows [][]string, defaultCourier string, defaultService string) []Row {
	var zoneColumns []struct {
		index int
		zone  string
	}
	for index, header := range headerRow {
		if zone := zoneFromString(header); zone != "" {
			zoneColumns = append(zoneColumns, struct {
				index int
				zone  string
			}{index: index, zone: zone})
		}
	}
	if len(zoneColumns) == 0 {
		return nil
	}

	weightIndex := 0
	if zoneColumns[0].index == 0 {
		weightIndex = 1
	}

	var parsed []Row
	for _, row := range rows {
		weight := strings.TrimSpace(cell(row, weightIndex))
		if weight == "" {
			continue
		}
		for _, column := range zoneColumns {
			if column.index == weightIndex {
				continue
			}
			price := priceFromString(cell(row, column.index))
			if price < 1 {
				continue
			}
			parsed = append(parsed, Row{
				Courier: firstNonEmpty(defaultCourier, "Custom courier"),
				Service: firstNonEmpty(defaultService, "Duty paid premium"),
				Zone:    column.zone,
				Weight:  weight,
				Price:   price,
			})
		}
	}
	return parsed
}

func cleanRow(row []string) []string {
	out := make([]string, len(row))
	for index, value := range row {
		out[index] = strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
	}
	return out
}

func normalizedHeaders(row []string) []string {
	out := make([]string, len(row))
	for index, value := range row {
		out[index] = normalizeHeader(value)
	}
	return out
}

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "/", "", ".", "", "(", "", ")", "")
	return replacer.Replace(value)
}

func hasAny(headers []string, values ...string) bool {
	return firstHeader(headers, values...) >= 0
}

func firstHeader(headers []string, values ...string) int {
	for index, header := range headers {
		for _, value := range values {
			if header == value || strings.Contains(header, value) {
				return index
			}
		}
	}
	return -1
}

func zoneHeaderCount(headers []string) int {
	count := 0
	for _, header := range headers {
		if zoneFromString(header) != "" {
			count++
		}
	}
	return count
}

var zonePattern = regexp.MustCompile(`(?i)(?:zone|z)?\s*([0-9]{1,2})\b`)

func zoneFromString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	match := zonePattern.FindStringSubmatch(value)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

var digitsPattern = regexp.MustCompile(`[0-9]+(?:[.,][0-9]+)?`)

func priceFromString(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	match := digitsPattern.FindString(value)
	if match == "" {
		return 0
	}
	match = strings.ReplaceAll(match, ",", "")
	amount, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return int(amount + 0.5)
}

func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

type sharedStringsXML struct {
	Items []struct {
		Text string `xml:"t"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

type worksheetXML struct {
	Rows []sheetRow `xml:"sheetData>row"`
}

type sheetRow struct {
	Cells []sheetCell `xml:"c"`
}

type sheetCell struct {
	Ref   string `xml:"r,attr"`
	Type  string `xml:"t,attr"`
	Value string `xml:"v"`
	Text  string `xml:"is>t"`
}

func readXLSX(raw []byte) ([][]string, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}

	shared := []string{}
	var worksheet *zip.File
	for _, file := range reader.File {
		switch {
		case file.Name == "xl/sharedStrings.xml":
			values, err := readSharedStrings(file)
			if err != nil {
				return nil, err
			}
			shared = values
		case strings.HasPrefix(file.Name, "xl/worksheets/") && strings.HasSuffix(file.Name, ".xml") && worksheet == nil:
			worksheet = file
		}
	}
	if worksheet == nil {
		return nil, errors.New("xlsx worksheet was not found")
	}
	return readWorksheet(worksheet, shared)
}

func readSharedStrings(file *zip.File) ([]string, error) {
	closer, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var payload sharedStringsXML
	if err := xml.NewDecoder(closer).Decode(&payload); err != nil {
		return nil, fmt.Errorf("read shared strings: %w", err)
	}

	values := make([]string, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.Text != "" {
			values = append(values, item.Text)
			continue
		}
		var builder strings.Builder
		for _, run := range item.Runs {
			builder.WriteString(run.Text)
		}
		values = append(values, builder.String())
	}
	return values, nil
}

func readWorksheet(file *zip.File, shared []string) ([][]string, error) {
	closer, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var payload worksheetXML
	if err := xml.NewDecoder(closer).Decode(&payload); err != nil {
		return nil, fmt.Errorf("read worksheet: %w", err)
	}

	var rows [][]string
	for _, sheetRow := range payload.Rows {
		row := []string{}
		for nextIndex, cell := range sheetRow.Cells {
			index := cellIndex(cell.Ref)
			if index < 0 {
				index = nextIndex
			}
			for len(row) <= index {
				row = append(row, "")
			}
			row[index] = cellValue(cell, shared)
		}
		rows = append(rows, cleanRow(row))
	}
	return rows, nil
}

func cellValue(cell sheetCell, shared []string) string {
	if cell.Type == "s" {
		index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err == nil && index >= 0 && index < len(shared) {
			return shared[index]
		}
	}
	if cell.Type == "inlineStr" {
		return cell.Text
	}
	return cell.Value
}

func cellIndex(ref string) int {
	index := 0
	seen := false
	for _, char := range ref {
		if char < 'A' || char > 'Z' {
			break
		}
		seen = true
		index = index*26 + int(char-'A'+1)
	}
	if !seen {
		return -1
	}
	return index - 1
}
