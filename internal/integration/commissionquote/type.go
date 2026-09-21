package commissionquote

import "github.com/shopspring/decimal"

type QuoteRequest struct {
	LoanAmount       decimal.Decimal `json:"loanAmount"`
	LoanTermInMonths int             `json:"loanTermInMonths"`
	RiskBand         string          `json:"riskBand"`
}

type QuoteResponse struct {
	QuoteID         string          `json:"quoteId"`
	CommissionRate  decimal.Decimal `json:"commissionRate"`
	TotalCommission decimal.Decimal `json:"totalCommission"`
}
