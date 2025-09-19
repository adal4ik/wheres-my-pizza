package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"
)

var (
	ErrRequeue = errors.New("requeue")     // nack(requeue=true)
	ErrDLQ     = errors.New("dead_letter") // nack(requeue=false)
)

type KitchenServiceInterface interface {
	Heartbeat(ctx context.Context) error
	Run(ctx context.Context) error
}

type KitchenService struct {
	db        repository.KitchenRepositoryInterface
	rmqClient rabbitmq.Client // у тебя клиент по значению — оставляю так

	WorkerName string
	WorkerType string

	Queue       string        // например, "orders_queue"
	OrderTypes  []string      // специализация воркера (если пусто — обрабатывает все типы)
	Prefetch    int           // basic.qos prefetch
	BeatEvery   time.Duration // интервал heartbeat
	UseConfirms bool          // ждать publisher confirms (опционально)

	// cooking times
	CookDineIn   time.Duration
	CookTakeout  time.Duration
	CookDelivery time.Duration

	pubAcks <-chan amqp091.Confirmation
}

// NewKitchenService — удобный конструктор с базовыми дефолтами.
func NewKitchenService(db repository.KitchenRepositoryInterface, rmqClient rabbitmq.Client, workerName string, orderTypesCSV string, prefetch int, heartbeatInterval int) KitchenServiceInterface {
	types := make([]string, 0)
	if strings.TrimSpace(orderTypesCSV) != "" {
		for _, t := range strings.Split(orderTypesCSV, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				types = append(types, t)
			}
		}
	}
	if prefetch <= 0 {
		prefetch = 1
	}
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30
	}
	return &KitchenService{
		db:           db,
		rmqClient:    rmqClient,
		WorkerName:   workerName,
		WorkerType:   "generic",
		Queue:        "orders_queue",
		OrderTypes:   types,
		Prefetch:     prefetch,
		BeatEvery:    time.Duration(heartbeatInterval) * time.Second,
		UseConfirms:  false,
		CookDineIn:   8 * time.Second,
		CookTakeout:  10 * time.Second,
		CookDelivery: 12 * time.Second,
	}
}

func (ks *KitchenService) Heartbeat(ctx context.Context) error {
	return ks.db.Heartbeat(ctx, ks.WorkerName)
}

func (ks *KitchenService) Run(ctx context.Context) error {
	// Валидация базовых параметров
	if strings.TrimSpace(ks.WorkerName) == "" {
		return fmt.Errorf("worker name is empty: pass --worker-name")
	}
	if strings.TrimSpace(ks.Queue) == "" {
		return fmt.Errorf("queue name is empty")
	}

	// Регистрация воркера (защита от дублей online)
	existedOnline, err := ks.db.RegisterOrFail(ctx, ks.WorkerName, ks.WorkerType)
	if err != nil {
		log.Printf(`{"level":"ERROR","action":"worker_registration_failed","err":%q}`, err.Error())
		if existedOnline {
			return err
		}
		return err
	}
	log.Printf(`{"level":"INFO","action":"worker_registered","name":%q,"type":%q}`, ks.WorkerName, ks.WorkerType)

	// Канал для consume
	consCh := ks.rmqClient.Channel()

	// Диагностика закрытий канала/консюмера
	closeCh := consCh.NotifyClose(make(chan *amqp091.Error, 1))
	cancelCh := consCh.NotifyCancel(make(chan string, 1))
	go func() {
		for {
			select {
			case e := <-closeCh:
				if e != nil {
					log.Printf(`{"level":"ERROR","action":"amqp_channel_closed","code":%d,"reason":%q}`, e.Code, e.Reason)
				}
				return
			case tag := <-cancelCh:
				if tag != "" {
					log.Printf(`{"level":"ERROR","action":"consumer_canceled","tag":%q}`, tag)
				}
			}
		}
	}()

	// Объявим инфраструктуру на всякий случай (идемпотентно)
	if err := consCh.ExchangeDeclare("notifications_fanout", "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare notifications_fanout: %w", err)
	}
	if _, err := consCh.QueueDeclarePassive(ks.Queue, true, false, false, false, nil); err != nil {
		if _, err := consCh.QueueDeclare(ks.Queue, true, false, false, false, amqp091.Table{"x-max-priority": int32(10)}); err != nil {
			return fmt.Errorf("queue declare %s: %w", ks.Queue, err)
		}
		if err := consCh.QueueBind(ks.Queue, "kitchen.*.*", "orders_topic", false, nil); err != nil {
			return fmt.Errorf("queue bind %s: %w", ks.Queue, err)
		}
	}

	// Prefetch
	if err := consCh.Qos(ks.Prefetch, 0, false); err != nil {
		return err
	}

	// Подписка
	consumerTag := ks.WorkerName
	msgs, err := consCh.Consume(ks.Queue, consumerTag, false, false, false, false, nil)
	if err != nil {
		return err
	}

	// Канал для публикаций (лучше не смешивать с consume-каналом)
	pubCh := ks.rmqClient.Channel()
	defer pubCh.Close()

	if ks.UseConfirms {
		if err := pubCh.Confirm(false); err != nil {
			return fmt.Errorf("confirm mode: %w", err)
		}
		ks.pubAcks = pubCh.NotifyPublish(make(chan amqp091.Confirmation, 1))
	}

	// Heartbeat
	stopBeat := make(chan struct{})
	go func() {
		t := time.NewTicker(ks.BeatEvery)
		defer t.Stop()
		for {
			select {
			case <-stopBeat:
				return
			case <-ctx.Done():
				return
			case <-t.C:
				if err := ks.db.Heartbeat(context.Background(), ks.WorkerName); err == nil {
					log.Printf(`{"level":"DEBUG","action":"heartbeat_sent","worker":%q}`, ks.WorkerName)
				}
			}
		}
	}()

	log.Printf(`[kitchen] consuming from %s (prefetch=%d, worker=%s)`, ks.Queue, ks.Prefetch, ks.WorkerName)

	// Основной цикл
	done := make(chan struct{})
	go func() {
		defer close(done)
		for d := range msgs {
			err := ks.processOne(ctx, pubCh, d)
			switch {
			case err == nil:
				_ = d.Ack(false)
			case errors.Is(err, ErrRequeue):
				_ = d.Nack(false, true)
			case errors.Is(err, ErrDLQ):
				_ = d.Nack(false, false)
			default:
				_ = d.Nack(false, true)
			}
		}
	}()

	// Ожидаем останов
	<-ctx.Done()
	log.Printf(`{"level":"INFO","action":"graceful_shutdown","worker":%q}`, ks.WorkerName)

	_ = consCh.Cancel(consumerTag, false)                     // перестаём принимать новые
	_ = ks.db.SetOffline(context.Background(), ks.WorkerName) // отметим offline
	close(stopBeat)
	<-done // дождёмся дренажа

	return nil
}

// ===== входящее сообщение из orders_topic, которое кладётся в orders_queue =====

type OrderItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

type OrderPayload struct {
	OrderNumber     string      `json:"order_number"`
	CustomerName    string      `json:"customer_name"`
	OrderType       string      `json:"order_type"` // dine_in|takeout|delivery
	TableNumber     int         `json:"table_number"`
	DeliveryAddress string      `json:"delivery_address"`
	Items           []OrderItem `json:"items"`
	TotalAmount     float64     `json:"total_amount"`
	Priority        int         `json:"priority"`
}

func (ks *KitchenService) allowedType(t string) bool {
	if len(ks.OrderTypes) == 0 {
		return true
	}
	t = strings.TrimSpace(strings.ToLower(t))
	for _, v := range ks.OrderTypes {
		if t == strings.TrimSpace(strings.ToLower(v)) {
			return true
		}
	}
	return false
}

func (ks *KitchenService) processOne(ctx context.Context, pubCh *amqp091.Channel, d amqp091.Delivery) error {
	var msg OrderPayload
	if err := json.Unmarshal(d.Body, &msg); err != nil {
		// нерепарабельный формат — в DLQ
		return ErrDLQ
	}
	if msg.OrderNumber == "" || msg.OrderType == "" {
		// без идентификатора/типа обработать корректно не сможем
		return ErrDLQ
	}
	if !ks.allowedType(msg.OrderType) {
		// не наша специализация — отдаём другому воркеру
		return ErrRequeue
	}

	// 1) received -> cooking (идемпотентно, транзакционно)
	ok, _, err := ks.db.TryStartCookingTx(ctx, msg.OrderNumber, ks.WorkerName)
	if err != nil {
		return ErrRequeue
	}
	if !ok {
		// уже не 'received' — идемпотентный повтор
		return nil
	}

	// 1b) уведомление "received -> cooking" (с ETA)
	if err := ks.publishStatus(ctx, pubCh,
		msg.OrderNumber, "received", "cooking", ks.WorkerName, ks.estimateFinish(msg.OrderType),
	); err != nil {
		return ErrRequeue
	}
	log.Printf(`{"level":"DEBUG","action":"order_processing_started","order_number":%q,"worker":%q}`, msg.OrderNumber, ks.WorkerName)

	// 2) имитация готовки
	select {
	case <-time.After(ks.cookDelayFor(msg.OrderType)):
	case <-ctx.Done():
		return ErrRequeue
	}

	// 3) cooking -> ready (транзакционно)
	if err := ks.db.MarkReadyTx(ctx, msg.OrderNumber, ks.WorkerName); err != nil {
		return ErrRequeue
	}

	// 3b) уведомление "cooking -> ready"
	if err := ks.publishStatus(ctx, pubCh,
		msg.OrderNumber, "cooking", "ready", ks.WorkerName, time.Now().UTC(),
	); err != nil {
		return ErrRequeue
	}

	log.Printf(`{"level":"DEBUG","action":"order_completed","order_number":%q,"worker":%q}`, msg.OrderNumber, ks.WorkerName)
	return nil
}

func (ks *KitchenService) cookDelayFor(typ string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "dine_in":
		return ks.CookDineIn
	case "takeout":
		return ks.CookTakeout
	case "delivery":
		return ks.CookDelivery
	default:
		return ks.CookTakeout
	}
}

func (ks *KitchenService) estimateFinish(typ string) time.Time {
	return time.Now().UTC().Add(ks.cookDelayFor(typ))
}

type statusMsg struct {
	OrderNumber         string    `json:"order_number"`
	OldStatus           string    `json:"old_status"`
	NewStatus           string    `json:"new_status"`
	ChangedBy           string    `json:"changed_by"`
	Timestamp           time.Time `json:"timestamp"`
	EstimatedCompletion time.Time `json:"estimated_completion"`
}

// публикация строго через PublishWithContext
func (ks *KitchenService) publishStatus(ctx context.Context, ch *amqp091.Channel,
	orderNumber, oldSt, newSt, worker string, eta time.Time) error {

	body, err := json.Marshal(statusMsg{
		OrderNumber:         orderNumber,
		OldStatus:           oldSt,
		NewStatus:           newSt,
		ChangedBy:           worker,
		Timestamp:           time.Now().UTC(),
		EstimatedCompletion: eta,
	})
	if err != nil {
		return err
	}

	pub := amqp091.Publishing{
		DeliveryMode:  amqp091.Persistent,
		ContentType:   "application/json",
		MessageId:     newMsgID(),
		CorrelationId: orderNumber,
		Timestamp:     time.Now().UTC(),
		Headers: amqp091.Table{
			"x-source": "kitchen",
			"x-worker": worker,
		},
		Body: body,
	}

	if err := ch.PublishWithContext(
		ctx,
		"notifications_fanout", // fanout exchange
		"",                     // routing key игнорируется
		false,
		false,
		pub,
	); err != nil {
		return err
	}

	if ks.UseConfirms && ks.pubAcks != nil {
		select {
		case conf := <-ks.pubAcks:
			if !conf.Ack {
				return fmt.Errorf("publish NACK from broker")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func newMsgID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
