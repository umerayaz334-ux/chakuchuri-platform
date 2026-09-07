package ratesheets

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type workbookSheet struct {
	Name string
	Rows [][]string
}

type workbookXML struct {
	Sheets []struct {
		Name string `xml:"name,attr"`
		RID  string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

type relationshipsXML struct {
	Relationships []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

var (
	etaPattern        = regexp.MustCompile(`(?i)ESTIMATED\s+([0-9]+)\s+TO\s+([0-9]+)\s+DAYS`)
	weightToPattern   = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*TO\s*([0-9]+(?:\.[0-9]+)?)`)
	weightPlusPattern = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*\+`)
	divisorPattern    = regexp.MustCompile(`(?i)/\s*([0-9]{3,6})`)
	boxWeightPattern  = regexp.MustCompile(`(?i)MAXIMUM\s+BOX\s+WEIGHT\s*([0-9]+(?:\.[0-9]+)?)`)
)

func ParseCatalog(raw []byte, filename string, options ParseOptions) (RateBook, error) {
	if strings.ToLower(filepath.Ext(filename)) != ".xlsx" {
		return RateBook{}, errors.New("guided shipping import currently requires an XLSX workbook")
	}
	sheets, err := readXLSXWorkbook(raw)
	if err != nil {
		return RateBook{}, err
	}
	carrier := strings.TrimSpace(options.Carrier)
	if carrier == "" {
		return RateBook{}, errors.New("carrier name is required")
	}
	currency := strings.ToUpper(strings.TrimSpace(options.Currency))
	if currency == "" {
		currency = "PKR"
	}
	if currency != "PKR" && currency != "USD" && currency != "EUR" && currency != "GBP" {
		return RateBook{}, errors.New("unsupported rate currency")
	}

	book := RateBook{
		Name: strings.TrimSpace(options.Name), Carrier: carrier, Country: "US", Currency: currency,
		DimensionalDivisor: 5000, MaxBoxWeightKG: 21.7, SpecialApprovalWeightKG: 22,
		SpecialPostalPrefixes: []string{"967", "968", "995", "996", "997", "998", "999"},
		Services:              []RateService{}, PostalZones: []PostalZone{}, Surcharges: []Surcharge{}, Warnings: []string{},
	}
	if book.Name == "" {
		book.Name = carrier + " USA rates"
	}

	var canonicalZones map[string]string
	for _, sheet := range sheets {
		service, metadata, zones, parseErr := parseCatalogSheet(sheet)
		if parseErr != nil {
			continue
		}
		book.Services = append(book.Services, service)
		if book.EffectiveDate == "" {
			book.EffectiveDate = metadata.effectiveDate
		} else if metadata.effectiveDate != "" && metadata.effectiveDate != book.EffectiveDate {
			book.Warnings = append(book.Warnings, "Service sheets contain different effective dates; the earliest detected workbook date is shown.")
			if metadata.effectiveDate < book.EffectiveDate {
				book.EffectiveDate = metadata.effectiveDate
			}
		}
		if metadata.divisor > 0 {
			book.DimensionalDivisor = metadata.divisor
		}
		if metadata.maxBoxWeight > 0 {
			book.MaxBoxWeightKG = metadata.maxBoxWeight
		}
		if len(metadata.flightDays) > 0 {
			book.FlightDays = metadata.flightDays
		}
		if len(book.Surcharges) == 0 {
			book.Surcharges = metadata.surcharges
		}
		if canonicalZones == nil {
			canonicalZones = zones
			book.PostalZones = postalZonesFromMap(zones)
		} else if !sameZoneMap(canonicalZones, zones) {
			book.Warnings = append(book.Warnings, "ZIP-zone mappings differ between service sheets; the first valid service map was retained.")
		}
	}

	if len(book.Services) == 0 || len(book.PostalZones) == 0 {
		return RateBook{}, ErrNoRates
	}
	if book.EffectiveDate == "" {
		book.Warnings = append(book.Warnings, "No effective date was detected; confirm the workbook version before activating it.")
	}
	for index := range book.Services {
		book.Services[index].ID = "preview_" + safeCatalogID(book.Services[index].Name)
	}
	book.Status = StatusPaused
	book.Warnings = uniqueStrings(book.Warnings)
	return book, nil
}

type sheetMetadata struct {
	effectiveDate string
	divisor       float64
	maxBoxWeight  float64
	flightDays    []string
	surcharges    []Surcharge
}

func parseCatalogSheet(sheet workbookSheet) (RateService, sheetMetadata, map[string]string, error) {
	service := RateService{Name: titleWords(sheet.Name), FixedRates: []FixedRate{}, Bands: []WeightBand{}}
	metadata := sheetMetadata{divisor: 5000, maxBoxWeight: 21.7, surcharges: []Surcharge{}}
	if service.Name == "" {
		service.Name = "Service"
	}
	for _, row := range sheet.Rows {
		joined := strings.Join(row, " ")
		if metadata.effectiveDate == "" {
			metadata.effectiveDate = dateFromCells(row)
		}
		if match := etaPattern.FindStringSubmatch(joined); len(match) == 3 {
			service.ETAMinDays, _ = strconv.Atoi(match[1])
			service.ETAMaxDays, _ = strconv.Atoi(match[2])
		}
		if strings.Contains(strings.ToUpper(joined), "DIMENSION") {
			if match := divisorPattern.FindStringSubmatch(joined); len(match) == 2 {
				metadata.divisor, _ = strconv.ParseFloat(match[1], 64)
			}
		}
		if match := boxWeightPattern.FindStringSubmatch(joined); len(match) == 2 {
			metadata.maxBoxWeight, _ = strconv.ParseFloat(match[1], 64)
		}
		upper := strings.ToUpper(joined)
		if strings.Contains(upper, "WEDNESDAY") && strings.Contains(upper, "THURSDAY") && strings.Contains(upper, "SATURDAY") {
			metadata.flightDays = []string{"Wednesday", "Thursday", "Saturday"}
		}
	}

	firstHeader := findZoneHeader(sheet.Rows, 0)
	if firstHeader < 0 {
		return RateService{}, sheetMetadata{}, nil, ErrNoRates
	}
	nonDutyRow := findContaining(sheet.Rows, "NON-DUTY", firstHeader+1)
	postalMarker := findContaining(sheet.Rows, "SEARCH FIRST THREE", firstHeader+1)
	if nonDutyRow < 0 {
		nonDutyRow = postalMarker
	}
	if postalMarker < 0 {
		postalMarker = len(sheet.Rows)
	}

	parseRateRows(&service, sheet.Rows[firstHeader+1:nonDutyRow], "duty_paid")
	if nonDutyRow >= 0 && nonDutyRow < len(sheet.Rows) {
		nonDutyHeader := findZoneHeader(sheet.Rows, nonDutyRow+1)
		if nonDutyHeader >= 0 && nonDutyHeader < postalMarker {
			parseRateRows(&service, sheet.Rows[nonDutyHeader+1:postalMarker], "non_duty_paid")
		}
	}
	if len(service.FixedRates) == 0 || len(service.Bands) == 0 {
		return RateService{}, sheetMetadata{}, nil, ErrNoRates
	}

	zones := parsePostalZones(sheet.Rows, postalMarker)
	if len(zones) == 0 {
		return RateService{}, sheetMetadata{}, nil, errors.New("ZIP-zone mapping was not found")
	}
	metadata.surcharges = parseSurcharges(sheet.Rows)
	return service, metadata, zones, nil
}

func parseRateRows(service *RateService, rows [][]string, dutyMode string) {
	for _, row := range rows {
		label := strings.TrimSpace(cell(row, 0))
		if weight, ok := numericCell(label); ok && weight > 0 && weight <= 10 {
			for zoneIndex := 1; zoneIndex <= 7; zoneIndex++ {
				if amount := priceFromString(cell(row, zoneIndex)); amount > 0 {
					service.FixedRates = append(service.FixedRates, FixedRate{DutyMode: dutyMode, Zone: strconv.Itoa(zoneIndex), WeightKG: weight, Amount: amount})
				}
			}
			if specialWeight, specialOK := numericCell(cell(row, 9)); specialOK {
				if amount := priceFromString(cell(row, 10)); amount > 0 {
					service.FixedRates = append(service.FixedRates, FixedRate{DutyMode: dutyMode, Zone: "HI_AK", WeightKG: specialWeight, Amount: amount})
				}
			}
			continue
		}
		minWeight, maxWeight, openEnded, ok := weightRange(label)
		if !ok {
			continue
		}
		for zoneIndex := 1; zoneIndex <= 7; zoneIndex++ {
			if amount := priceFromString(cell(row, zoneIndex)); amount > 0 {
				service.Bands = append(service.Bands, WeightBand{DutyMode: dutyMode, Zone: strconv.Itoa(zoneIndex), MinWeightKG: minWeight, MaxWeightKG: maxWeight, OpenEnded: openEnded, AmountPerKG: amount})
			}
		}
		if specialMin, specialMax, specialOpen, specialOK := weightRange(cell(row, 9)); specialOK {
			if amount := priceFromString(cell(row, 10)); amount > 0 {
				service.Bands = append(service.Bands, WeightBand{DutyMode: dutyMode, Zone: "HI_AK", MinWeightKG: specialMin, MaxWeightKG: specialMax, OpenEnded: specialOpen, AmountPerKG: amount})
			}
		}
	}
	sort.SliceStable(service.FixedRates, func(i, j int) bool {
		if service.FixedRates[i].DutyMode != service.FixedRates[j].DutyMode {
			return service.FixedRates[i].DutyMode < service.FixedRates[j].DutyMode
		}
		if service.FixedRates[i].Zone != service.FixedRates[j].Zone {
			return service.FixedRates[i].Zone < service.FixedRates[j].Zone
		}
		return service.FixedRates[i].WeightKG < service.FixedRates[j].WeightKG
	})
}

func parsePostalZones(rows [][]string, marker int) map[string]string {
	if marker < 0 || marker >= len(rows) {
		return nil
	}
	header := findZoneHeader(rows, marker+1)
	if header < 0 {
		return nil
	}
	zoneColumns := map[int]string{}
	for index, value := range rows[header] {
		if zone := zoneFromString(value); zone != "" {
			zoneColumns[index] = zone
		}
	}
	result := map[string]string{}
	for _, row := range rows[header+1:] {
		for index, zone := range zoneColumns {
			prefix := normalizedPrefix(cell(row, index))
			if prefix == "" {
				continue
			}
			if existing, exists := result[prefix]; exists && existing != zone {
				continue
			}
			result[prefix] = zone
		}
	}
	return result
}

func parseSurcharges(rows [][]string) []Surcharge {
	result := []Surcharge{}
	for _, row := range rows {
		joined := strings.Join(row, " ")
		upper := strings.ToUpper(joined)
		switch {
		case strings.Contains(upper, "PSW") && strings.Contains(upper, "2500"):
			result = append(result, Surcharge{Key: "psw", Label: "PSW handling", Amount: 2500, Currency: "PKR", Description: "Per-shipment handling charge shown in the uploaded workbook."})
		case strings.Contains(upper, "ADDRESS CORRECTION") && strings.Contains(upper, "25.5"):
			result = append(result, Surcharge{Key: "address_correction", Label: "Address correction", Amount: 25.5, Currency: "USD", Description: "Minimum charge when the courier corrects an address."})
		case strings.Contains(upper, "MAX BOX WEIGHT") && strings.Contains(upper, "60.00"):
			result = append(result, Surcharge{Key: "oversize", Label: "Oversize package", Amount: 60, Currency: "USD", Description: "Minimum per-box charge when a weight or dimension limit is exceeded."})
		}
	}
	return uniqueSurcharges(result)
}

func readXLSXWorkbook(raw []byte) ([]workbookSheet, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("read xlsx: %w", err)
	}
	files := map[string]*zip.File{}
	worksheetNames := []string{}
	for _, file := range reader.File {
		files[file.Name] = file
		if strings.HasPrefix(file.Name, "xl/worksheets/") && strings.HasSuffix(file.Name, ".xml") {
			worksheetNames = append(worksheetNames, file.Name)
		}
	}
	shared := []string{}
	if file := files["xl/sharedStrings.xml"]; file != nil {
		shared, err = readSharedStrings(file)
		if err != nil {
			return nil, err
		}
	}

	ordered := []struct{ name, target string }{}
	if workbookFile := files["xl/workbook.xml"]; workbookFile != nil {
		var workbook workbookXML
		if err := decodeZipXML(workbookFile, &workbook); err != nil {
			return nil, err
		}
		relTargets := map[string]string{}
		if relsFile := files["xl/_rels/workbook.xml.rels"]; relsFile != nil {
			var rels relationshipsXML
			if err := decodeZipXML(relsFile, &rels); err != nil {
				return nil, err
			}
			for _, relation := range rels.Relationships {
				target := strings.TrimPrefix(path.Clean(strings.TrimPrefix(relation.Target, "/")), "./")
				if !strings.HasPrefix(target, "xl/") {
					target = path.Clean("xl/" + target)
				}
				relTargets[relation.ID] = target
			}
		}
		for _, sheet := range workbook.Sheets {
			if target := relTargets[sheet.RID]; target != "" {
				ordered = append(ordered, struct{ name, target string }{sheet.Name, target})
			}
		}
	}
	if len(ordered) == 0 {
		sort.Strings(worksheetNames)
		for index, target := range worksheetNames {
			ordered = append(ordered, struct{ name, target string }{fmt.Sprintf("Sheet %d", index+1), target})
		}
	}

	result := make([]workbookSheet, 0, len(ordered))
	for _, entry := range ordered {
		file := files[entry.target]
		if file == nil {
			continue
		}
		rows, readErr := readWorksheet(file, shared)
		if readErr != nil {
			return nil, readErr
		}
		result = append(result, workbookSheet{Name: entry.name, Rows: rows})
	}
	if len(result) == 0 {
		return nil, errors.New("xlsx worksheets were not found")
	}
	return result, nil
}

func decodeZipXML(file *zip.File, target interface{}) error {
	closer, err := file.Open()
	if err != nil {
		return err
	}
	defer closer.Close()
	if err := xml.NewDecoder(closer).Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode %s: %w", file.Name, err)
	}
	return nil
}

func findZoneHeader(rows [][]string, start int) int {
	for index := maxInt(start, 0); index < len(rows); index++ {
		if zoneHeaderCount(normalizedHeaders(rows[index])) >= 7 {
			return index
		}
	}
	return -1
}

func findContaining(rows [][]string, text string, start int) int {
	text = strings.ToUpper(text)
	for index := maxInt(start, 0); index < len(rows); index++ {
		if strings.Contains(strings.ToUpper(strings.Join(rows[index], " ")), text) {
			return index
		}
	}
	return -1
}

func dateFromCells(row []string) string {
	for _, value := range row {
		value = strings.TrimSpace(value)
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02", "02-Jan-2006", "01/02/2006"} {
			if parsed, err := time.Parse(layout, value); err == nil && parsed.Year() >= 2000 && parsed.Year() <= 2200 {
				return parsed.Format("2006-01-02")
			}
		}
		if serial, err := strconv.ParseFloat(value, 64); err == nil && serial > 30000 && serial < 100000 {
			return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(math.Round(serial*24)) * time.Hour).Format("2006-01-02")
		}
	}
	return ""
}

func numericCell(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	parsed, err := strconv.ParseFloat(value, 64)
	return parsed, err == nil
}

func weightRange(value string) (float64, float64, bool, bool) {
	value = strings.TrimSpace(value)
	if match := weightToPattern.FindStringSubmatch(value); len(match) == 3 {
		minimum, _ := strconv.ParseFloat(match[1], 64)
		maximum, _ := strconv.ParseFloat(match[2], 64)
		return minimum, maximum, false, true
	}
	if match := weightPlusPattern.FindStringSubmatch(value); len(match) == 2 {
		minimum, _ := strconv.ParseFloat(match[1], 64)
		return minimum, 0, true, true
	}
	return 0, 0, false, false
}

func normalizedPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		integer := int(math.Round(number))
		if integer >= 0 && integer <= 999 {
			return fmt.Sprintf("%03d", integer)
		}
	}
	var digits strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}
	if digits.Len() == 0 || digits.Len() > 3 {
		return ""
	}
	return fmt.Sprintf("%03s", digits.String())
}

func postalZonesFromMap(values map[string]string) []PostalZone {
	prefixes := make([]string, 0, len(values))
	for prefix := range values {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)
	result := make([]PostalZone, 0, len(prefixes))
	for _, prefix := range prefixes {
		result = append(result, PostalZone{Country: "US", Prefix: prefix, Zone: values[prefix]})
	}
	return result
}

func sameZoneMap(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for prefix, zone := range left {
		if right[prefix] != zone {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func uniqueSurcharges(values []Surcharge) []Surcharge {
	seen := map[string]bool{}
	result := []Surcharge{}
	for _, value := range values {
		if value.Key == "" || seen[value.Key] {
			continue
		}
		seen[value.Key] = true
		result = append(result, value)
	}
	return result
}

func titleWords(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	words := strings.Fields(value)
	for index := range words {
		if len(words[index]) > 0 {
			words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
		}
	}
	return strings.Join(words, " ")
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}
