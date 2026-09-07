package workflow

import (
	"encoding/json"

	"chakuchuri/backend/internal/auth"
)

// A failed save must not leave in-memory balances ahead of durable state.
// This checkpoint is transitional while the workflow uses snapshot persistence.
type financialState struct {
	Quotes   []Quotation
	Orders   []ManufacturingOrder
	Shipping []ShippingRequest
	Payments []Payment
	Ledger   []LedgerEntry
	Day      string
	Sequence int
}

type financialMutation struct {
	service   *Service
	before    financialState
	committed bool
}

func (s *Service) beginFinancialMutationLocked() (*financialMutation, error) {
	state := financialState{s.quotations, s.manufacturing, s.shipping, s.payments, s.ledger, s.lastSequenceDay, s.sequence}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, errPersistence
	}
	mutation := &financialMutation{service: s}
	if err := json.Unmarshal(raw, &mutation.before); err != nil {
		return nil, errPersistence
	}
	return mutation, nil
}

func (m *financialMutation) rollback() {
	if m.committed {
		return
	}
	s, before := m.service, m.before
	s.quotations, s.manufacturing, s.shipping = before.Quotes, before.Orders, before.Shipping
	s.payments, s.ledger = before.Payments, before.Ledger
	s.lastSequenceDay, s.sequence = before.Day, before.Sequence
}

func (m *financialMutation) commit(actor auth.User) error {
	if err := m.service.saveStateLocked(); err != nil {
		return err
	}
	m.committed = true
	previous := make(map[string]bool, len(m.before.Ledger))
	for _, entry := range m.before.Ledger {
		previous[entry.ID] = true
	}
	for _, entry := range m.service.ledger {
		if !previous[entry.ID] {
			m.service.record(actor, "ledger.posted", entry.EntryNo, entry.Note)
		}
	}
	return nil
}
