package models

// Transaction merepresentasikan satu baris tabel transactions.
type Transaction struct {
	TransactionID   string  `json:"transaction_id"`
	ProductID       string  `json:"product_id"`
	ProductName     string  `json:"product_name"`
	Quantity        int     `json:"quantity"`
	UnitPrice       float64 `json:"unit_price"`
	DiscountPercent float64 `json:"discount_percent"`
	TotalPrice      float64 `json:"total_price"`
	PaymentMethod   string  `json:"payment_method"`
	TransactionDate string  `json:"transaction_date"`
	CashierName     string  `json:"cashier_name"`
}

// TransactionItemInput adalah satu item keranjang.
type TransactionItemInput struct {
	ProductID       string  `json:"product_id"`
	Quantity        int     `json:"quantity"`
	DiscountPercent float64 `json:"discount_percent"`
}

// TransactionInput adalah body JSON untuk POST /api/transactions.
// Mendukung banyak item (items) maupun satu item (product_id + quantity).
type TransactionInput struct {
	Items           []TransactionItemInput `json:"items"`
	ProductID       string                 `json:"product_id"`
	Quantity        int                    `json:"quantity"`
	DiscountPercent float64                `json:"discount_percent"`
	PaymentMethod   string                 `json:"payment_method"`
	TransactionDate string                 `json:"transaction_date"`
}
