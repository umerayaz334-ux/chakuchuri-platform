package ratesheets

import (
	"errors"
	"strings"
	"sync"

	"chakuchuri/backend/internal/auth"
)

type BookingRequest struct {
	Lookup        LookupRequest `json:"lookup"`
	RecipientName string        `json:"recipientName"`
	Phone         string        `json:"phone"`
	AddressLine1  string        `json:"addressLine1"`
	AddressLine2  string        `json:"addressLine2"`
	City          string        `json:"city"`
	State         string        `json:"state"`
	PostalCode    string        `json:"postalCode"`
	Contents      string        `json:"contents"`
}

type BookingPayload struct {
	Destination  string
	PostalPrefix string
	Contents     string
	RateBookID   string
	Option       LookupOption
}

type BookingCreator func(BookingPayload, auth.User) (interface{}, error)

var bookingCreators sync.Map

func (s *Service) SetBookingCreator(creator BookingCreator) {
	if creator == nil {
		bookingCreators.Delete(s)
		return
	}
	bookingCreators.Store(s, creator)
}

func (s *Service) Book(request BookingRequest, actor auth.User) (interface{}, error) {
	request.RecipientName = strings.TrimSpace(request.RecipientName)
	request.Phone = strings.TrimSpace(request.Phone)
	request.AddressLine1 = strings.TrimSpace(request.AddressLine1)
	request.AddressLine2 = strings.TrimSpace(request.AddressLine2)
	request.City = strings.TrimSpace(request.City)
	request.State = strings.TrimSpace(request.State)
	request.PostalCode = strings.TrimSpace(request.PostalCode)
	request.Contents = strings.TrimSpace(request.Contents)
	request.Lookup.Country = strings.ToUpper(strings.TrimSpace(request.Lookup.Country))
	if request.Lookup.Country == "" {
		request.Lookup.Country = "US"
	}
	if request.RecipientName == "" || request.Phone == "" || request.AddressLine1 == "" || request.City == "" || request.PostalCode == "" || request.Contents == "" {
		return nil, errors.New("recipient, phone, address, city, postal code and contents are required")
	}
	request.Lookup.PostalCode = request.PostalCode
	result, err := s.Lookup(request.Lookup)
	if err != nil {
		return nil, err
	}
	if len(result.Options) != 1 {
		return nil, errors.New("select one available shipping service before booking")
	}
	option := result.Options[0]
	creatorValue, ok := bookingCreators.Load(s)
	if !ok {
		return nil, errors.New("shipping booking is not connected")
	}
	creator, ok := creatorValue.(BookingCreator)
	if !ok || creator == nil {
		return nil, errors.New("shipping booking is not connected")
	}
	destinationParts := []string{request.RecipientName, request.AddressLine1}
	if request.AddressLine2 != "" {
		destinationParts = append(destinationParts, request.AddressLine2)
	}
	locality := request.City
	if request.State != "" {
		locality += ", " + request.State
	}
	locality += " " + request.PostalCode
	destinationParts = append(destinationParts, locality, request.Lookup.Country, request.Phone)
	return creator(BookingPayload{
		Destination: strings.Join(destinationParts, " / "), PostalPrefix: result.PostalPrefix,
		Contents: request.Contents, RateBookID: option.RateBookID, Option: option,
	}, actor)
}
