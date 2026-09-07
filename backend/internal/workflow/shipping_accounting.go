package workflow

import (
	"encoding/json"
	"strings"

	"chakuchuri/backend/internal/auth"
)

type ShippingPricing struct {
	RateBookID string          `json:"rateBookId"`
	ServiceID  string          `json:"serviceId"`
	Currency   string          `json:"currency"`
	Amount     int             `json:"amount"`
	Breakdown  json.RawMessage `json:"breakdown,omitempty"`
}

// Only the trusted catalog calculator calls this; pricing is not JSON-decodable
// on the legacy customer booking payload.
func (s *Service) CreateQuotedShipping(payload ShippingRequestPayload, price ShippingPricing, actor auth.User) (ShippingRequest, error) {
	if price.Amount < 1 || price.Amount > maxMoneyAmount || price.Currency != "PKR" || price.RateBookID == "" || price.ServiceID == "" {
		return ShippingRequest{}, errValidation
	}
	payload.pricing = &price
	return s.CreateShipping(payload, actor)
}

func (s *Service) matchingShippingRate(courier, service, zone, weight string) (int, error) {
	for _, rate := range s.rateSheets {
		if strings.EqualFold(rate.Courier, courier) && strings.EqualFold(rate.Service, service) && rate.Zone == zone && strings.EqualFold(rate.Weight, weight) && strings.EqualFold(rate.Status, "Active") && rate.Price > 0 && rate.Price <= maxMoneyAmount {
			return rate.Price, nil
		}
	}
	return 0, errValidation
}
