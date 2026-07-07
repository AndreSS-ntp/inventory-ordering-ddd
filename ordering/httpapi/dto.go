package httpapi

type createOrderRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type orderResponse struct {
	ID                string `json:"id"`
	SKU               string `json:"sku"`
	Quantity          int    `json:"quantity"`
	Status            string `json:"status"`
	FailureReason     string `json:"failure_reason,omitempty"`
	CreatedAt         string `json:"created_at"`
	InventoryAttempts int    `json:"inventory_attempts"`
}
