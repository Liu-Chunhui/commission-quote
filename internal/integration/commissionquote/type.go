package commissionquote

import (
	"context"

	"github.com/shopspring/decimal"
)

type Quoter interface {
	GenerateQuote(ctx context.Context, key string, input QuoteRequest) (QuoteResponse, error)
}

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
