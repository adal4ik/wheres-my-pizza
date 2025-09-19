package service

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"
	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"

	"github.com/rabbitmq/amqp091-go"
)

type KitchenServiceInterface interface {
	Heartbeat(ctx context.Context) error
	Run(ctx context.Context) error
}

type KitchenService struct {
	db          repository.KitchenRepositoryInterface
	rmqClient   rabbitmq.Client
	WorkerName  string
	Queue       string        // "orders_queue"
	OrderTypes  []string      // например: []{"dine_in","takeout","delivery"} или конкретные
	Prefetch    int           // 10
	CookDelayMS time.Duration // 2000
	MaxAttempts int           // 3..5 (если внедришь ретраи)
}

func NewKitchenService(db repository.KitchenRepositoryInterface, rmqClient rabbitmq.Client) KitchenServiceInterface {
	return &KitchenService{db: db, rmqClient: rmqClient}
}

func (ks *KitchenService) Heartbeat(ctx context.Context) error {
	return ks.db.Heartbeat(ctx, ks.WorkerName)
}

func (ks *KitchenService) Run(ctx context.Context) error {
	// зарегистрируем/обновим воркера
	if err := ks.db.UpsertWorker(ctx, ks.WorkerName, "generic"); err != nil {
		return err
	}

	ch := ks.rmqClient.Channel()
	if ks.Prefetch <= 0 {
		ks.Prefetch = 10
	}
	if err := ch.Qos(ks.Prefetch, 0, false); err != nil {
		return err
	}

	msgs, err := ch.Consume(
		ks.Queue, // "orders_queue"
		ks.WorkerName,
		false, // autoAck
		false, false, false, nil,
	)
	if err != nil {
		return err
	}

	log.Printf("[kitchen] consuming from %s, prefetch=%d", ks.Queue, ks.Prefetch)

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return errors.New("consumer channel closed")
			}
			if err := ks.handle(ctx, d); err != nil {
				log.Printf("[kitchen] handle error: %v", err)
				// Минимально: не зацикливаем — отклоняем без requeue (уйдёт в DLQ если настроено)
				_ = d.Nack(false, false)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

type OrderItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type OrderPayload struct {
	Event string `json:"event"`
	Order struct {
		ID          int64       `json:"id"`
		Type        string      `json:"type"`
		Priority    string      `json:"priority"`
		TotalAmount float64     `json:"total_amount"`
		Items       []OrderItem `json:"items"`
		CreatedAt   time.Time   `json:"created_at"`
	} `json:"order"`
}

func (ks *KitchenService) allowedType(t string) bool {
	if len(ks.OrderTypes) == 0 {
		return true
	}
	for _, v := range ks.OrderTypes {
		if v == t {
			return true
		}
	}
	return false
}

func (ks *KitchenService) handle(ctx context.Context, d amqp091.Delivery) error {
	var msg OrderPayload
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		return err
	}
	if msg.Event != "order.created" {
		// просто пропустим чужие события
		return nil
	}
	if !ks.allowedType(msg.Order.Type) {
		return nil
	}

	// 1) пытаемся «захватить» заказ: received -> cooking
	ok, err := ks.db.TryStartCookingTx(ctx, msg.Order.ID, ks.WorkerName)
	if err != nil {
		return err
	}
	if !ok {
		// кто-то уже взял или статус не received — просто идем дальше
		return nil
	}

	// 2) имитируем готовку
	if ks.CookDelayMS <= 0 {
		ks.CookDelayMS = 2 * time.Second
	}
	select {
	case <-time.After(ks.CookDelayMS):
	case <-ctx.Done():
		return ctx.Err()
	}

	// 3) переводим в ready -> completed (можно в 2 шага, тут упростим)
	if err := ks.db.MarkReadyTx(ctx, msg.Order.ID, ks.WorkerName); err != nil {
		return err
	}
	if err := ks.db.MarkCompletedTx(ctx, msg.Order.ID, ks.WorkerName); err != nil {
		return err
	}

	// здесь при желании можно шлёпнуть нотификацию в fanout
	// (у тебя в def.json пока нет fanout — пропущу)

	return nil
}
