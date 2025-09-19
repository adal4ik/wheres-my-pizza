package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"wheres-my-pizza/internal/connections/rabbitmq"
	"wheres-my-pizza/internal/microservices/kitchen/repository"
)

var (
	ErrRequeue = errors.New("requeue")
	ErrDLQ     = errors.New("dead_letter")
)

const (
	attachRetries    = 60
	attachSleep      = time.Second
	pollIdleSleep    = 150 * time.Millisecond
	heartbeatDefault = 30 * time.Second
)

type KitchenServiceInterface interface {
	Heartbeat(ctx context.Context) error
	Run(ctx context.Context) error
}

type KitchenService struct {
	db        repository.KitchenRepositoryInterface
	rmqClient rabbitmq.Client

	WorkerName string
	WorkerType string // "", "dine_in", "takeout", "delivery"

	CookDineIn   time.Duration
	CookTakeout  time.Duration
	CookDelivery time.Duration

	BeatEvery   time.Duration
	UseConfirms bool

	pubAcks <-chan amqp091.Confirmation
}

func NewKitchenService(
	db repository.KitchenRepositoryInterface,
	rmqClient rabbitmq.Client,
	workerName, workerType string,
	_ int, // prefetch (unused with GET mode)
	heartbeatSeconds int,
) KitchenServiceInterface {
	if heartbeatSeconds <= 0 {
		heartbeatSeconds = int(heartbeatDefault / time.Second)
	}
	return &KitchenService{
		db:           db,
		rmqClient:    rmqClient,
		WorkerName:   workerName,
		WorkerType:   strings.TrimSpace(workerType),
		CookDineIn:   8 * time.Second,
		CookTakeout:  10 * time.Second,
		CookDelivery: 12 * time.Second,
		BeatEvery:    time.Duration(heartbeatSeconds) * time.Second,
		UseConfirms:  false,
	}
}

func (ks *KitchenService) Heartbeat(ctx context.Context) error {
	return ks.db.Heartbeat(ctx, ks.WorkerName)
}

type OrderItem struct {
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}
type OrderPayload struct {
	OrderNumber     string      `json:"order_number"`
	CustomerName    string      `json:"customer_name"`
	OrderType       string      `json:"order_type"`
	TableNumber     int         `json:"table_number"`
	DeliveryAddress string      `json:"delivery_address"`
	Items           []OrderItem `json:"items"`
	TotalAmount     float64     `json:"total_amount"`
	Priority        int         `json:"priority"`
}

func (ks *KitchenService) Run(ctx context.Context) error {
	if strings.TrimSpace(ks.WorkerName) == "" {
		return fmt.Errorf("worker name is empty: pass --worker-name")
	}
	dupOnline, err := ks.db.RegisterOrWakeUp(ctx, ks.WorkerName, ks.workerTypeOrAll())
	if err != nil {
		if dupOnline {
			log.Printf(`{"level":"ERROR","action":"worker_registration_failed","err":%q}`, err.Error())
		}
		return err
	}
	log.Printf(`{"level":"INFO","action":"worker_registered","name":%q,"type":%q}`, ks.WorkerName, ks.workerTypeOrAll())

	// publisher (on its own channel; auto-reconnect handled by client)
	pubCh := ks.rmqClient.Channel()
	defer pubCh.Close()
	if err := pubCh.ExchangeDeclare("notifications_fanout", "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare notifications_fanout: %w", err)
	}
	if ks.UseConfirms {
		if err := pubCh.Confirm(false); err != nil {
			return fmt.Errorf("confirm mode: %w", err)
		}
		ks.pubAcks = pubCh.NotifyPublish(make(chan amqp091.Confirmation, 1))
	}

	// GET channel (sequential)
	getCh := ks.rmqClient.Channel()
	defer getCh.Close()

	queues := ks.activeQueues()
	if err := ks.waitQueues(ctx, getCh, queues...); err != nil {
		return err
	}

	// heartbeat
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

	log.Printf(`[kitchen] started round-robin GET, worker=%s, type=%s`, ks.WorkerName, ks.workerTypeOrAll())

	idx := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf(`{"level":"INFO","action":"graceful_shutdown","worker":%q}`, ks.WorkerName)
			_ = ks.db.SetOffline(context.Background(), ks.WorkerName)
			close(stopBeat)
			return nil
		default:
		}

		got := false
		for i := 0; i < len(queues); i++ {
			q := queues[idx%len(queues)]
			idx++

			d, ok, err := getCh.Get(q, false)
			if err != nil {
				// channel/connection glitch → reopen and retry on next loop
				log.Printf(`{"level":"WARN","action":"basic_get_failed","queue":%q,"err":%q}`, q, err.Error())
				_ = getCh.Close()
				getCh = ks.rmqClient.Channel()
				_ = ks.waitQueues(ctx, getCh, queues...) // re-verify
				continue
			}
			if !ok {
				continue
			}
			got = true

			var o OrderPayload
			if err := json.Unmarshal(d.Body, &o); err != nil {
				_ = d.Nack(false, false)
				log.Printf(`{"level":"ERROR","action":"parse_failed","queue":%q,"err":%q}`, q, err.Error())
				break
			}

			// process
			var perr error
			switch strings.ToLower(o.OrderType) {
			case "dine_in":
				perr = ks.processCommon(ctx, pubCh, o, ks.CookDineIn)
			case "takeout":
				perr = ks.processCommon(ctx, pubCh, o, ks.CookTakeout)
			case "delivery":
				perr = ks.processCommon(ctx, pubCh, o, ks.CookDelivery)
			default:
				perr = ErrDLQ
			}

			switch {
			case perr == nil:
				_ = d.Ack(false)
			case errors.Is(perr, ErrRequeue):
				rq := retryQueueFor(o.OrderType)
				if e := ks.toRetry(ctx, pubCh, d, rq, 5); e != nil {
					log.Printf(`{"level":"ERROR","action":"retry_publish_failed","queue":%q,"err":%q}`, rq, e.Error())
					_ = d.Nack(false, true)
				} else {
					_ = d.Ack(false)
				}
			case errors.Is(perr, ErrDLQ):
				_ = d.Nack(false, false)
			default:
				_ = d.Nack(false, true)
			}

			break
		}

		if !got {
			select {
			case <-ctx.Done():
				_ = ks.db.SetOffline(context.Background(), ks.WorkerName)
				close(stopBeat)
				return nil
			case <-time.After(pollIdleSleep):
			}
		}
	}
}

// ---- helpers ----

func (ks *KitchenService) activeQueues() []string {
	switch strings.ToLower(strings.TrimSpace(ks.WorkerType)) {
	case "dine_in":
		return []string{"kitchen.dine_in.q"}
	case "takeout":
		return []string{"kitchen.takeout.q"}
	case "delivery":
		return []string{"kitchen.delivery.q"}
	default:
		return []string{"kitchen.dine_in.q", "kitchen.takeout.q", "kitchen.delivery.q"}
	}
}

func (ks *KitchenService) waitQueues(ctx context.Context, ch *amqp091.Channel, queues ...string) error {
	for _, q := range queues {
		var err error
		ok := false
		for i := 0; i < attachRetries; i++ {
			if _, err = ch.QueueDeclarePassive(q, true, false, false, false, nil); err == nil {
				ok = true
				break
			}
			log.Printf(`{"level":"WARN","action":"wait_queue","queue":%q,"err":%q,"try":%d}`, q, err.Error(), i+1)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(attachSleep):
			}
		}
		if !ok {
			return fmt.Errorf("queue %s not ready: %w", q, err)
		}
	}
	return nil
}

// ---- processing + notifications ----

func (ks *KitchenService) processCommon(ctx context.Context, pubCh *amqp091.Channel, o OrderPayload, cook time.Duration) error {
	if o.OrderNumber == "" {
		return ErrDLQ
	}
	ok, _, err := ks.db.TryStartCookingTx(ctx, o.OrderNumber, ks.WorkerName)
	if err != nil {
		return ErrRequeue
	}
	if !ok {
		return nil
	}
	if err := ks.publishStatus(ctx, pubCh, o.OrderNumber, "received", "cooking", ks.WorkerName, time.Now().UTC().Add(cook)); err != nil {
		return ErrRequeue
	}
	log.Printf(`{"level":"DEBUG","action":"order_processing_started","order_number":%q,"worker":%q}`, o.OrderNumber, ks.WorkerName)

	select {
	case <-time.After(cook):
	case <-ctx.Done():
		return ErrRequeue
	}

	if err := ks.db.MarkReadyTx(ctx, o.OrderNumber, ks.WorkerName); err != nil {
		return ErrRequeue
	}
	if err := ks.publishStatus(ctx, pubCh, o.OrderNumber, "cooking", "ready", ks.WorkerName, time.Now().UTC()); err != nil {
		return ErrRequeue
	}

	log.Printf(`{"level":"DEBUG","action":"order_completed","order_number":%q,"worker":%q}`, o.OrderNumber, ks.WorkerName)
	return nil
}

func (ks *KitchenService) toRetry(ctx context.Context, pubCh *amqp091.Channel, d amqp091.Delivery, retryQueue string, max int) error {
	cnt := getRetryCount(d) + 1
	if cnt > max {
		return ErrDLQ
	}
	h := d.Headers
	if h == nil {
		h = amqp091.Table{}
	}
	h["x-retry-count"] = cnt

	pub := amqp091.Publishing{
		DeliveryMode:  amqp091.Persistent,
		ContentType:   d.ContentType,
		Body:          d.Body,
		MessageId:     d.MessageId,
		CorrelationId: d.CorrelationId,
		Timestamp:     time.Now().UTC(),
		Headers:       h,
		Priority:      d.Priority,
	}
	return pubCh.PublishWithContext(ctx, "", retryQueue, false, false, pub)
}

func getRetryCount(d amqp091.Delivery) int {
	if d.Headers == nil {
		return 0
	}
	if v, ok := d.Headers["x-retry-count"]; ok {
		switch t := v.(type) {
		case int:
			return t
		case int32:
			return int(t)
		case int64:
			return int(t)
		case string:
			if n, err := strconv.Atoi(t); err == nil {
				return n
			}
		}
	}
	if d.Redelivered {
		return 1
	}
	return 0
}

func retryQueueFor(orderType string) string {
	switch strings.ToLower(strings.TrimSpace(orderType)) {
	case "dine_in":
		return "kitchen.dine_in.retry.q"
	case "takeout":
		return "kitchen.takeout.retry.q"
	case "delivery":
		return "kitchen.delivery.retry.q"
	default:
		return "kitchen.takeout.retry.q"
	}
}

type statusMsg struct {
	OrderNumber         string    `json:"order_number"`
	OldStatus           string    `json:"old_status"`
	NewStatus           string    `json:"new_status"`
	ChangedBy           string    `json:"changed_by"`
	Timestamp           time.Time `json:"timestamp"`
	EstimatedCompletion time.Time `json:"estimated_completion"`
}

func (ks *KitchenService) publishStatus(ctx context.Context, ch *amqp091.Channel,
	orderNumber, oldSt, newSt, worker string, eta time.Time,
) error {
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

	if err := ch.PublishWithContext(ctx, "notifications_fanout", "", false, false, pub); err != nil {
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

func (ks *KitchenService) workerTypeOrAll() string {
	if ks.WorkerType == "" {
		return "all"
	}
	return ks.WorkerType
}

func newMsgID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
