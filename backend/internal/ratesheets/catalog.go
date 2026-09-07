package ratesheets

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chakuchuri/backend/internal/audit"
	"chakuchuri/backend/internal/auth"
)

const (
	StatusActive = "Active"
	StatusPaused = "Paused"
)

var (
	errBookNotFound  = errors.New("shipping rate book not found")
	errInvalidLookup = errors.New("shipping rate lookup is invalid")
)

type FixedRate struct {
	DutyMode string  `json:"dutyMode"`
	Zone     string  `json:"zone"`
	WeightKG float64 `json:"weightKg"`
	Amount   int     `json:"amount"`
}

type WeightBand struct {
	DutyMode    string  `json:"dutyMode"`
	Zone        string  `json:"zone"`
	MinWeightKG float64 `json:"minWeightKg"`
	MaxWeightKG float64 `json:"maxWeightKg,omitempty"`
	OpenEnded   bool    `json:"openEnded,omitempty"`
	AmountPerKG int     `json:"amountPerKg"`
}

type RateService struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	ETAMinDays int          `json:"etaMinDays"`
	ETAMaxDays int          `json:"etaMaxDays"`
	FixedRates []FixedRate  `json:"fixedRates"`
	Bands      []WeightBand `json:"bands"`
}

type PostalZone struct {
	Country string `json:"country"`
	Prefix  string `json:"prefix"`
	Zone    string `json:"zone"`
}

type Surcharge struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Description string  `json:"description,omitempty"`
}

type RateBook struct {
	ID                      string        `json:"id"`
	Name                    string        `json:"name"`
	Carrier                 string        `json:"carrier"`
	Country                 string        `json:"country"`
	Currency                string        `json:"currency"`
	EffectiveDate           string        `json:"effectiveDate"`
	Version                 int           `json:"version"`
	Status                  string        `json:"status"`
	SourceFileID            string        `json:"sourceFileId,omitempty"`
	SourceName              string        `json:"sourceName,omitempty"`
	ImportedAt              string        `json:"importedAt"`
	ImportedBy              string        `json:"importedBy"`
	DimensionalDivisor      float64       `json:"dimensionalDivisor"`
	MaxBoxWeightKG          float64       `json:"maxBoxWeightKg"`
	SpecialApprovalWeightKG float64       `json:"specialApprovalWeightKg,omitempty"`
	FlightDays              []string      `json:"flightDays,omitempty"`
	SpecialPostalPrefixes   []string      `json:"specialPostalPrefixes"`
	Services                []RateService `json:"services"`
	PostalZones             []PostalZone  `json:"postalZones"`
	Surcharges              []Surcharge   `json:"surcharges"`
	Warnings                []string      `json:"warnings,omitempty"`
}

type ZoneCount struct {
	Zone  string `json:"zone"`
	Count int    `json:"count"`
}

type ServiceSummary struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ETAMinDays     int    `json:"etaMinDays"`
	ETAMaxDays     int    `json:"etaMaxDays"`
	FixedRateCount int    `json:"fixedRateCount"`
	BandCount      int    `json:"bandCount"`
	MinimumAmount  int    `json:"minimumAmount"`
}

type BookSummary struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Carrier            string           `json:"carrier"`
	Country            string           `json:"country"`
	Currency           string           `json:"currency"`
	EffectiveDate      string           `json:"effectiveDate"`
	Version            int              `json:"version"`
	Status             string           `json:"status"`
	SourceFileID       string           `json:"sourceFileId,omitempty"`
	SourceName         string           `json:"sourceName,omitempty"`
	ImportedAt         string           `json:"importedAt"`
	ImportedBy         string           `json:"importedBy"`
	DimensionalDivisor float64          `json:"dimensionalDivisor"`
	MaxBoxWeightKG     float64          `json:"maxBoxWeightKg"`
	PostalPrefixCount  int              `json:"postalPrefixCount"`
	SpecialPrefixCount int              `json:"specialPrefixCount"`
	Services           []ServiceSummary `json:"services"`
	ZoneCounts         []ZoneCount      `json:"zoneCounts"`
	Surcharges         []Surcharge      `json:"surcharges"`
	Warnings           []string         `json:"warnings,omitempty"`
}

type Snapshot struct {
	Books           []BookSummary `json:"books"`
	ActiveBooks     int           `json:"activeBooks"`
	ActiveServices  int           `json:"activeServices"`
	LatestEffective string        `json:"latestEffective,omitempty"`
	Pagination      PageInfo      `json:"pagination"`
}

type PageInfo struct {
	Loaded  int  `json:"loaded"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

type ParseOptions struct {
	Carrier  string
	Name     string
	Currency string
}

type ImportPreview struct {
	Book BookSummary `json:"book"`
}

type LookupRequest struct {
	Country    string  `json:"country"`
	PostalCode string  `json:"postalCode"`
	DutyMode   string  `json:"dutyMode"`
	WeightKG   float64 `json:"weightKg"`
	LengthCM   float64 `json:"lengthCm"`
	WidthCM    float64 `json:"widthCm"`
	HeightCM   float64 `json:"heightCm"`
	Packages   int     `json:"packages"`
	IncludePSW bool    `json:"includePsw"`
	Carrier    string  `json:"carrier,omitempty"`
	ServiceID  string  `json:"serviceId,omitempty"`
}

type ChargeLine struct {
	Label    string  `json:"label"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type LookupOption struct {
	RateBookID         string       `json:"rateBookId"`
	ServiceID          string       `json:"serviceId"`
	Carrier            string       `json:"carrier"`
	Service            string       `json:"service"`
	Currency           string       `json:"currency"`
	EffectiveDate      string       `json:"effectiveDate"`
	ETAMinDays         int          `json:"etaMinDays"`
	ETAMaxDays         int          `json:"etaMaxDays"`
	Zone               string       `json:"zone"`
	Region             string       `json:"region"`
	DutyMode           string       `json:"dutyMode"`
	Packages           int          `json:"packages"`
	ActualWeightKG     float64      `json:"actualWeightKg"`
	VolumetricWeightKG float64      `json:"volumetricWeightKg"`
	ChargeableWeightKG float64      `json:"chargeableWeightKg"`
	RateMode           string       `json:"rateMode"`
	RateWeightKG       float64      `json:"rateWeightKg"`
	UnitRate           int          `json:"unitRate"`
	BaseAmount         int          `json:"baseAmount"`
	Charges            []ChargeLine `json:"charges"`
	TotalAmount        int          `json:"totalAmount"`
	RequiresReview     bool         `json:"requiresReview"`
	ReviewReasons      []string     `json:"reviewReasons,omitempty"`
}

type LookupResponse struct {
	PostalPrefix string         `json:"postalPrefix"`
	Zone         string         `json:"zone"`
	Region       string         `json:"region"`
	Options      []LookupOption `json:"options"`
	Warnings     []string       `json:"warnings,omitempty"`
}

type Service struct {
	mu         sync.RWMutex
	recorder   *audit.Recorder
	repository Repository
	books      []RateBook
}

func NewService(recorder *audit.Recorder) *Service {
	return &Service{recorder: recorder, books: []RateBook{}}
}

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return snapshotFromBooks(s.books)
}

func PreviewBook(book RateBook) ImportPreview {
	return ImportPreview{Book: summarizeBook(book)}
}

func (s *Service) Import(book RateBook, sourceFileID string, sourceName string, activate bool, actor auth.User) (BookSummary, error) {
	if len(book.Services) == 0 || len(book.PostalZones) == 0 || strings.TrimSpace(book.Carrier) == "" {
		return BookSummary{}, ErrNoRates
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	version := 1
	for index := range s.books {
		if !strings.EqualFold(s.books[index].Carrier, book.Carrier) || !strings.EqualFold(s.books[index].Country, book.Country) {
			continue
		}
		if s.books[index].Version >= version {
			version = s.books[index].Version + 1
		}
		if activate && s.books[index].Status == StatusActive {
			s.books[index].Status = StatusPaused
		}
	}

	book.ID = "ratebook_" + safeCatalogID(book.Carrier) + "_v" + strconv.Itoa(version) + "_" + now.Format("20060102T150405")
	book.Version = version
	book.Status = StatusPaused
	if activate {
		book.Status = StatusActive
	}
	book.SourceFileID = strings.TrimSpace(sourceFileID)
	book.SourceName = strings.TrimSpace(sourceName)
	book.ImportedAt = now.Format(time.RFC3339)
	book.ImportedBy = catalogActorName(actor)
	for index := range book.Services {
		book.Services[index].ID = book.ID + "_" + safeCatalogID(book.Services[index].Name)
	}

	s.books = append([]RateBook{book}, s.books...)
	s.persistLocked()
	s.record(actor, "shipping_rates.imported", book.ID, fmt.Sprintf("Imported %d services and %d postal prefixes from %s.", len(book.Services), len(book.PostalZones), book.SourceName))
	return summarizeBook(book), nil
}

func (s *Service) SetActive(id string, active bool, actor auth.User) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	target := -1
	for index := range s.books {
		if s.books[index].ID == strings.TrimSpace(id) {
			target = index
			break
		}
	}
	if target < 0 {
		return Snapshot{}, errBookNotFound
	}
	if active {
		for index := range s.books {
			if index != target && strings.EqualFold(s.books[index].Carrier, s.books[target].Carrier) && strings.EqualFold(s.books[index].Country, s.books[target].Country) {
				s.books[index].Status = StatusPaused
			}
		}
		s.books[target].Status = StatusActive
	} else {
		s.books[target].Status = StatusPaused
	}
	s.persistLocked()
	s.record(actor, "shipping_rates.status_updated", s.books[target].ID, "Shipping rate book status changed to "+s.books[target].Status+".")
	return snapshotFromBooks(s.books), nil
}

func (s *Service) Lookup(request LookupRequest) (LookupResponse, error) {
	request.Country = strings.ToUpper(strings.TrimSpace(request.Country))
	if request.Country == "" {
		request.Country = "US"
	}
	request.DutyMode = normalizeDutyMode(request.DutyMode)
	if request.DutyMode == "" {
		request.DutyMode = "duty_paid"
	}
	if request.Packages < 1 {
		request.Packages = 1
	}
	if request.WeightKG <= 0 || request.WeightKG > 1000 || request.Packages > 100 || request.LengthCM < 0 || request.WidthCM < 0 || request.HeightCM < 0 {
		return LookupResponse{}, errInvalidLookup
	}
	prefix := postalPrefix(request.PostalCode, request.Country)
	if prefix == "" {
		if request.Country == "US" {
			return LookupResponse{}, fmt.Errorf("%w: enter a valid US ZIP code", errInvalidLookup)
		}
		return LookupResponse{}, fmt.Errorf("%w: enter a valid postal code", errInvalidLookup)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	response := LookupResponse{PostalPrefix: prefix, Options: []LookupOption{}, Warnings: []string{}}
	foundDestination := false
	for _, book := range s.books {
		if book.Status != StatusActive || !strings.EqualFold(book.Country, request.Country) {
			continue
		}
		if request.Carrier != "" && !strings.EqualFold(book.Carrier, request.Carrier) {
			continue
		}

		zone, region, found := destinationFor(book, prefix)
		if !found {
			continue
		}
		foundDestination = true
		response.Zone = zone
		response.Region = region
		for _, rateService := range book.Services {
			if request.ServiceID != "" && rateService.ID != request.ServiceID {
				continue
			}
			option, ok := calculateOption(book, rateService, zone, region, request)
			if ok {
				response.Options = append(response.Options, option)
			}
		}
	}
	if !foundDestination {
		response.Warnings = append(response.Warnings, "This postal area is not covered by an active rate book. Ask the shipping team for a manual quote.")
	} else if len(response.Options) == 0 {
		if request.DutyMode == "non_duty_paid" && request.WeightKG <= 10 {
			response.Warnings = append(response.Warnings, "This workbook only provides non-duty-paid rates above 10 kg. Ask the shipping team for a manual quote.")
		} else {
			response.Warnings = append(response.Warnings, "No matching service rate was found for this weight.")
		}
	}
	sort.SliceStable(response.Options, func(i, j int) bool {
		if response.Options[i].TotalAmount == response.Options[j].TotalAmount {
			return response.Options[i].ETAMinDays < response.Options[j].ETAMinDays
		}
		return response.Options[i].TotalAmount < response.Options[j].TotalAmount
	})
	return response, nil
}

func calculateOption(book RateBook, service RateService, zone string, region string, request LookupRequest) (LookupOption, bool) {
	divisor := book.DimensionalDivisor
	if divisor <= 0 {
		divisor = 5000
	}
	volumetric := 0.0
	if request.LengthCM > 0 && request.WidthCM > 0 && request.HeightCM > 0 {
		volumetric = request.LengthCM * request.WidthCM * request.HeightCM / divisor
	}
	chargeable := math.Max(request.WeightKG, volumetric)
	rateWeight := math.Ceil(chargeable*2-1e-9) / 2
	if rateWeight > 10 {
		rateWeight = math.Ceil(chargeable - 1e-9)
		if rateWeight < 11 {
			rateWeight = 11
		}
	}

	rateZone := zone
	if region == "Hawaii / Alaska" {
		rateZone = "HI_AK"
	}
	unitRate := 0
	rateMode := "fixed"
	basePerPackage := 0
	if rateWeight <= 10 {
		selectedWeight := math.MaxFloat64
		for _, rate := range service.FixedRates {
			if rate.DutyMode == request.DutyMode && rate.Zone == rateZone && rate.WeightKG+1e-9 >= rateWeight {
				if rate.WeightKG < selectedWeight {
					unitRate = rate.Amount
					selectedWeight = rate.WeightKG
				}
			}
		}
		if selectedWeight < math.MaxFloat64 {
			rateWeight = selectedWeight
		}
		basePerPackage = unitRate
	} else {
		rateMode = "per_kg"
		bestMin := -1.0
		for _, band := range service.Bands {
			if band.DutyMode != request.DutyMode || band.Zone != rateZone || rateWeight+1e-9 < band.MinWeightKG {
				continue
			}
			if !band.OpenEnded && band.MaxWeightKG > 0 && rateWeight-1e-9 > band.MaxWeightKG {
				continue
			}
			if band.MinWeightKG >= bestMin {
				bestMin = band.MinWeightKG
				unitRate = band.AmountPerKG
			}
		}
		basePerPackage = int(math.Ceil(float64(unitRate) * rateWeight))
	}
	if unitRate <= 0 {
		return LookupOption{}, false
	}

	baseAmount := basePerPackage * request.Packages
	charges := []ChargeLine{}
	total := baseAmount
	if request.IncludePSW {
		for _, surcharge := range book.Surcharges {
			if surcharge.Key == "psw" && strings.EqualFold(surcharge.Currency, book.Currency) {
				amount := int(math.Round(surcharge.Amount))
				charges = append(charges, ChargeLine{Label: surcharge.Label, Amount: surcharge.Amount, Currency: surcharge.Currency})
				total += amount
				break
			}
		}
	}

	reviewReasons := packageReviewReasons(book, request)
	return LookupOption{
		RateBookID: book.ID, ServiceID: service.ID, Carrier: book.Carrier, Service: service.Name,
		Currency: book.Currency, EffectiveDate: book.EffectiveDate, ETAMinDays: service.ETAMinDays, ETAMaxDays: service.ETAMaxDays,
		Zone: zone, Region: region, DutyMode: request.DutyMode, Packages: request.Packages,
		ActualWeightKG: roundMeasure(request.WeightKG), VolumetricWeightKG: roundMeasure(volumetric), ChargeableWeightKG: roundMeasure(chargeable),
		RateMode: rateMode, RateWeightKG: rateWeight, UnitRate: unitRate, BaseAmount: baseAmount, Charges: charges, TotalAmount: total,
		RequiresReview: len(reviewReasons) > 0, ReviewReasons: reviewReasons,
	}, true
}

func packageReviewReasons(book RateBook, request LookupRequest) []string {
	reasons := []string{}
	maxWeight := book.MaxBoxWeightKG
	if maxWeight <= 0 {
		maxWeight = 21.7
	}
	if request.WeightKG > maxWeight {
		reasons = append(reasons, fmt.Sprintf("Each box must be %.1f kg or lighter; this package needs approval or splitting.", maxWeight))
	}
	dimensions := []float64{request.LengthCM, request.WidthCM, request.HeightCM}
	sort.Sort(sort.Reverse(sort.Float64Slice(dimensions)))
	if dimensions[0] > 120 {
		reasons = append(reasons, "The longest side is above 120 cm and may receive an oversize charge.")
	}
	if dimensions[1] > 70 {
		reasons = append(reasons, "The second-longest side is above 70 cm and may receive an oversize charge.")
	}
	if dimensions[0]+2*dimensions[1]+2*dimensions[2] > 260 {
		reasons = append(reasons, "Package girth is above 260 cm and may receive an oversize charge.")
	}
	return reasons
}

func destinationFor(book RateBook, prefix string) (string, string, bool) {
	for _, special := range book.SpecialPostalPrefixes {
		if special == prefix {
			region := "Special destination"
			if strings.EqualFold(book.Country, "US") {
				region = "Hawaii / Alaska"
			}
			return "Special", region, true
		}
	}
	for _, mapping := range book.PostalZones {
		if mapping.Prefix == prefix {
			region := strings.ToUpper(strings.TrimSpace(book.Country))
			if region == "US" {
				region = "Continental USA"
			}
			return mapping.Zone, region, true
		}
	}
	return "", "", false
}

func snapshotFromBooks(books []RateBook) Snapshot {
	snapshot := Snapshot{Books: make([]BookSummary, 0, len(books))}
	for _, book := range books {
		summary := summarizeBook(book)
		snapshot.Books = append(snapshot.Books, summary)
		if book.Status == StatusActive {
			snapshot.ActiveBooks++
			snapshot.ActiveServices += len(book.Services)
			if book.EffectiveDate > snapshot.LatestEffective {
				snapshot.LatestEffective = book.EffectiveDate
			}
		}
	}
	return snapshot
}

func summarizeBook(book RateBook) BookSummary {
	counts := map[string]int{}
	for _, mapping := range book.PostalZones {
		counts[mapping.Zone]++
	}
	zones := make([]ZoneCount, 0, len(counts))
	for zone, count := range counts {
		zones = append(zones, ZoneCount{Zone: zone, Count: count})
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].Zone < zones[j].Zone })

	services := make([]ServiceSummary, 0, len(book.Services))
	for _, service := range book.Services {
		minimum := 0
		for _, rate := range service.FixedRates {
			if rate.Amount > 0 && (minimum == 0 || rate.Amount < minimum) {
				minimum = rate.Amount
			}
		}
		services = append(services, ServiceSummary{
			ID: service.ID, Name: service.Name, ETAMinDays: service.ETAMinDays, ETAMaxDays: service.ETAMaxDays,
			FixedRateCount: len(service.FixedRates), BandCount: len(service.Bands), MinimumAmount: minimum,
		})
	}
	return BookSummary{
		ID: book.ID, Name: book.Name, Carrier: book.Carrier, Country: book.Country, Currency: book.Currency,
		EffectiveDate: book.EffectiveDate, Version: book.Version, Status: book.Status, SourceFileID: book.SourceFileID,
		SourceName: book.SourceName, ImportedAt: book.ImportedAt, ImportedBy: book.ImportedBy,
		DimensionalDivisor: book.DimensionalDivisor, MaxBoxWeightKG: book.MaxBoxWeightKG,
		PostalPrefixCount: len(book.PostalZones), SpecialPrefixCount: len(book.SpecialPostalPrefixes), Services: services,
		ZoneCounts: zones, Surcharges: append([]Surcharge{}, book.Surcharges...), Warnings: append([]string{}, book.Warnings...),
	}
}

func postalPrefix(value string, country string) string {
	var normalized strings.Builder
	for _, char := range strings.ToUpper(strings.TrimSpace(value)) {
		if char >= '0' && char <= '9' {
			normalized.WriteRune(char)
			continue
		}
		if !strings.EqualFold(country, "US") && char >= 'A' && char <= 'Z' {
			normalized.WriteRune(char)
		}
	}
	if normalized.Len() < 3 {
		return ""
	}
	return normalized.String()[:3]
}

func normalizeDutyMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "duty_paid", "duty":
		return "duty_paid"
	case "non_duty_paid", "nonduty", "non_duty":
		return "non_duty_paid"
	default:
		return ""
	}
}

func safeCatalogID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	lastUnderscore := false
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			out.WriteRune(char)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			out.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(out.String(), "_")
}

func catalogActorName(actor auth.User) string {
	if strings.TrimSpace(actor.Name) != "" {
		return strings.TrimSpace(actor.Name)
	}
	if strings.TrimSpace(actor.Email) != "" {
		return strings.TrimSpace(actor.Email)
	}
	return "system"
}

func roundMeasure(value float64) float64 {
	return math.Round(value*100) / 100
}

func (s *Service) record(actor auth.User, action string, entity string, detail string) {
	if s.recorder != nil {
		s.recorder.Record(catalogActorName(actor), action, entity, detail)
	}
}
