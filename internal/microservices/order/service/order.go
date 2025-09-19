package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"wheres-my-pizza/internal/connections/rabbitmq"
	dao "wheres-my-pizza/internal/microservices/order/domain/dao"
	dto "wheres-my-pizza/internal/microservices/order/domain/dto"
	"wheres-my-pizza/internal/microservices/order/repository"

	"github.com/rabbitmq/amqp091-go"
)

type OrderServiceInterface interface {
	AddOrder(req dto.CreateOrderRequest) (dto.CreateOrderResponse, error)
}

type OrderService struct {
	db        repository.OrderRepositoryInterface
	rmqClient rabbitmq.Client
}

func NewOrderService(db repository.OrderRepositoryInterface, rmqClient rabbitmq.Client) OrderServiceInterface {
	return &OrderService{db: db, rmqClient: rmqClient}
}

func (or *OrderService) AddOrder(req dto.CreateOrderRequest) (dto.CreateOrderResponse, error) {
	// 1. Basic validation
	if req.CustomerName == "" {
		return dto.CreateOrderResponse{}, errors.New("customer name is required")
	}
	if req.OrderType != "dine_in" && req.OrderType != "takeout" && req.OrderType != "delivery" {
		return dto.CreateOrderResponse{}, errors.New("invalid order type")
	}
	if len(req.Items) == 0 {
		return dto.CreateOrderResponse{}, errors.New("at least one item is required")
	}

	// 2. Calculate total amount
	total := 0.0
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return dto.CreateOrderResponse{}, fmt.Errorf("invalid quantity for item %s", item.Name)
		}
		if item.Price <= 0 {
			return dto.CreateOrderResponse{}, fmt.Errorf("invalid price for item %s", item.Name)
		}
		total += float64(item.Quantity) * item.Price
	}
	priority := 0
	if total >= 100 {
		priority = 10
	} else if total >= 50 {
		priority = 5
	} else {
		priority = 1
	}
	// 3. Generate order number (ORD_YYYYMMDD_NNN)
	today := time.Now().UTC().Format("20060102")
	sequence, err := or.db.GetOrderCount()
	if err != nil {
		return dto.CreateOrderResponse{}, fmt.Errorf("failed to get order count: %w", err)
	}
	orderNumber := fmt.Sprintf("ORD_%s_%03d", today, sequence)

	// 4. Save order in database
	order := dao.Order{
		OrderNumber:  orderNumber,
		CustomerName: req.CustomerName,
		OrderType:    req.OrderType,
		Items:        dto.ConvertItems(req.Items),
		TotalAmount:  total,
		Status:       "received",
		Priority:     priority,
		// DeliveryAddr: req.DeliveryAddress,
		// TableNumber: req.TableNumber,
	}
	if err := or.db.AddOrder(order); err != nil {
		return dto.CreateOrderResponse{}, fmt.Errorf("failed to save order: %w", err)
	}

	// 5. Publish to RabbitMQ
	msg := dao.OrderMessage{
		OrderNumber:     order.OrderNumber,
		CustomerName:    order.CustomerName,
		OrderType:       order.OrderType,
		Items:           order.Items,
		TotalAmount:     order.TotalAmount,
		Priority:        order.Priority,
		DeliveryAddress: order.DeliveryAddr,
		TableNumber:     order.TableNumber,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return dto.CreateOrderResponse{}, fmt.Errorf("failed to marshal order message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	routingKey := fmt.Sprintf("kitchen.%s.%d", req.OrderType, priority)

	pub := amqp091.Publishing{
		DeliveryMode:  amqp091.Persistent, // persist to disk
		ContentType:   "application/json",
		Body:          body,
		MessageId:     fmt.Sprintf("%d", time.Now().UnixNano()),
		CorrelationId: orderNumber, // удобно трекать по номеру
		Timestamp:     time.Now().UTC(),
		Priority:      uint8(priority), // задействуем x-max-priority очереди
		Headers: amqp091.Table{
			"x-source": "order-service",
		},
	}

	if err := or.rmqClient.Channel().PublishWithContext(
		ctx,
		"orders_topic",
		routingKey,
		false, // mandatory
		false, // immediate
		pub,
	); err != nil {
		return dto.CreateOrderResponse{}, fmt.Errorf("failed to publish order: %w", err)
	}

	// 6. Build response
	resp := dto.CreateOrderResponse{
		OrderNumber: orderNumber,
		Status:      "received",
		TotalAmount: total,
	}
	return resp, nil
}
