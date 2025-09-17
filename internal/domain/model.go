package domain

type Order struct {
	ID           int
	Number       string
	CustomerName string
	Type         string // dine_in | takeout | delivery
	TableNumber  *int
	DeliveryAddr *string
	TotalAmount  float64
	Priority     int
	Status       string
	ProcessedBy  *string
}

type OrderItem struct {
	ID, OrderID, Quantity int
	Name                  string
	Price                 float64
}

type Worker struct {
	ID              int
	Name, Type      string
	Status          string
	OrdersProcessed int
}
