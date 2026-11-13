package domain

import "time"

// DomainEvent describes a business fact that has already happened.
// Persist events to event_outbox in the same transaction as the business change.
type DomainEvent interface {
	EventType() string
	OccurredAt() time.Time
}

type EventMetadata struct {
	At time.Time `json:"occurredAt"`
}

func (m EventMetadata) OccurredAt() time.Time { return m.At }

type PaymentRecorded struct {
	EventMetadata
	PaymentID   string `json:"paymentId"`
	MemberID    string `json:"memberId"`
	AmountMinor int64  `json:"amountMinor"`
	Currency    string `json:"currency"`
}

func (PaymentRecorded) EventType() string { return "scheme.payment_recorded.v1" }

type DrawResultRecorded struct {
	EventMetadata
	DrawID   string `json:"drawId"`
	MemberID string `json:"memberId"`
}

func (DrawResultRecorded) EventType() string { return "scheme.draw_result_recorded.v1" }
