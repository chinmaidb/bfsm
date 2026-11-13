package application

import "time"

type Actor struct {
	UserID    string
	Role      string
	RequestID string
}

type Promoter struct {
	ID      string `json:"id"`
	UserID  string `json:"userId,omitempty"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email,omitempty"`
	Address string `json:"address"`
	Active  bool   `json:"active"`
}

type Member struct {
	ID            string     `json:"id"`
	UserID        string     `json:"userId,omitempty"`
	PromoterID    string     `json:"promoterId"`
	MemberNumber  int64      `json:"memberNumber,omitempty"`
	Name          string     `json:"name"`
	Phone         string     `json:"phone"`
	Email         string     `json:"email,omitempty"`
	Address       string     `json:"address"`
	KYCType       string     `json:"kycType,omitempty"`
	KYCLast4      string     `json:"kycLast4,omitempty"`
	KYCVerifiedAt *time.Time `json:"kycVerifiedAt,omitempty"`
	Active        bool       `json:"active"`
}

type Scheme struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	ContributionMinor int64  `json:"contributionMinor"`
	DurationMonths    int    `json:"durationMonths"`
	Currency          string `json:"currency"`
	Active            bool   `json:"active"`
}

type Login struct {
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Permissions []string `json:"permissions"`
	IsSuperAdmin bool `json:"isSuperAdmin"`
	Token     string    `json:"accessToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PaymentInput struct {
	IdempotencyKey string `json:"idempotencyKey"`
	MemberID       string `json:"memberId"`
	EnrollmentID   string `json:"enrollmentId"`
	AmountMinor    int64  `json:"amountMinor"`
	Currency       string `json:"currency"`
	PaymentMethod  string `json:"paymentMethod"`
	Reference      string `json:"reference"`
}

type DrawInput struct {
	SchemeID  string `json:"schemeId"`
	DrawMonth string `json:"drawMonth"`
	MemberID  string `json:"memberId"`
	Notes     string `json:"notes"`
}

type Party struct {
	ID        string `json:"id"`
	PartyType string `json:"partyType"`
	MemberID  string `json:"memberId,omitempty"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email,omitempty"`
	Address   string `json:"address"`
	Active    bool   `json:"active"`
}

type FinancialEntryInput struct {
	IdempotencyKey string `json:"idempotencyKey"`
	PartyID        string `json:"partyId"`
	AmountMinor    int64  `json:"amountMinor"`
	Currency       string `json:"currency"`
	PaymentMethod  string `json:"paymentMethod"`
	OccurredAt     string `json:"occurredAt"`
	Reference      string `json:"reference"`
	Description    string `json:"description"`
}
