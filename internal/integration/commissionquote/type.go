package commissionquote

type QuoteRequest struct {
	LoanAmount       int    `json:"loanAmount"`
	LoanTermInMonths int    `json:"loanTermInMonths"`
	RiskBand         string `json:"riskBand"`
}

type QuoteResponse struct {
	QuoteID         string  `json:"quoteId"`
	CommissionRate  float64 `json:"commissionRate"`
	TotalCommission float64 `json:"totalCommission"`
}
