package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"wheres-my-pizza/internal/connections/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	exchangeName    = "notifications_fanout" // уже создан у тебя
	consumerQPrefix = "notifications_queue." // очередь на инстанс: notifications_queue.<hostname>
	prefetch        = 50
	autoAck         = false // ВАЖНО: ручной ack, как в ТЗ
	exclusiveQueue  = true  // чтобы каждый инстанс получал все сообщения
	autoDeleteQueue = true  // удалится при закрытии канала/коннекта
	durableQueue    = false // временная, не нужна durable
)

// ожидаемый формат сообщения
type StatusUpdate struct {
	OrderNumber string    `json:"order_number"`
	OldStatus   string    `json:"old_status"`
	NewStatus   string    `json:"new_status"`
	ChangedBy   string    `json:"changed_by"`
	ChangedAt   time.Time `json:"changed_at"`
}

type NotificatorService struct {
	rmqClient rabbitmq.Client
}

func NewNotificatorService(rmqClient rabbitmq.Client) *NotificatorService {
	return &NotificatorService{rmqClient: rmqClient}
}

func (ns *NotificatorService) Notify(ctx context.Context) error {
	ch := ns.rmqClient.Channel()

	if err := ch.ExchangeDeclare(
		exchangeName, "fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		logJSON("ERROR", "rabbitmq_connection_failed", "ExchangeDeclare failed", map[string]any{"error": err.Error()})
		return err
	}

	host, _ := os.Hostname()
	qName := consumerQPrefix + host
	q, err := ch.QueueDeclare(
		qName,
		durableQueue,
		autoDeleteQueue,
		exclusiveQueue,
		false,
		nil,
	)
	if err != nil {
		logJSON("ERROR", "rabbitmq_connection_failed", "QueueDeclare failed", map[string]any{"error": err.Error()})
		return err
	}

	if err := ch.QueueBind(q.Name, "", exchangeName, false, nil); err != nil {
		logJSON("ERROR", "rabbitmq_connection_failed", "QueueBind failed", map[string]any{"error": err.Error()})
		return err
	}

	if err := ch.Qos(prefetch, 0, false); err != nil {
		logJSON("ERROR", "rabbitmq_connection_failed", "Qos failed", map[string]any{"error": err.Error()})
		return err
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		autoAck,
		exclusiveQueue,
		false,
		false,
		nil,
	)
	if err != nil {
		logJSON("ERROR", "rabbitmq_connection_failed", "Consume failed", map[string]any{"error": err.Error()})
		return err
	}

	logJSON("INFO", "service_started", "Notification subscriber started", map[string]any{
		"exchange": exchangeName,
		"queue":    q.Name,
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				// канал закрыт — выходим
				return nil
			}
			if err := ns.handleDelivery(d); err != nil {
				_ = d.Nack(false, false)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

func (ns *NotificatorService) handleDelivery(d amqp.Delivery) error {
	var su StatusUpdate
	if err := json.Unmarshal(d.Body, &su); err != nil {
		logJSON("ERROR", "notification_parse_failed", "Bad message format", map[string]any{
			"error": err.Error(),
			"body":  string(d.Body),
		})
		return err
	}

	logJSON("DEBUG", "notification_received",
		fmt.Sprintf("Received status update for order %s", su.OrderNumber),
		map[string]any{
			"order_number": su.OrderNumber,
			"new_status":   su.NewStatus,
			"old_status":   su.OldStatus,
			"changed_by":   su.ChangedBy,
			"changed_at":   su.ChangedAt.Format(time.RFC3339),
		},
	)

	fmt.Printf(
		"Notification for order %s: Status changed from '%s' to '%s' by %s.\n",
		su.OrderNumber, su.OldStatus, su.NewStatus, su.ChangedBy,
	)
	return nil
}

type logEntry struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Service   string         `json:"service"`
	Hostname  string         `json:"hostname"`
	RequestID string         `json:"request_id,omitempty"`
	Action    string         `json:"action"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

func logJSON(level, action, msg string, details map[string]any) {
	host, _ := os.Hostname()
	le := logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		Service:   "notification-subscriber",
		Hostname:  host,
		Action:    action,
		Message:   msg,
		Details:   details,
	}
	b, _ := json.Marshal(le)
	log.Println(string(b))
}
