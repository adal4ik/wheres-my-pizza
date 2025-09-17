package mq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func Dial(host string, port int, user, pass string) (*Client, error) {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d/", user, pass, host, port)
	conn, err := amqp.Dial(url)
	if err != nil { return nil, err }
	ch, err := conn.Channel()
	if err != nil { conn.Close(); return nil, err }
	return &Client{conn: conn, ch: ch}, nil
}

func (c *Client) Close() {
	if c == nil { return }
	if c.ch != nil { _ = c.ch.Close() }
	if c.conn != nil { _ = c.conn.Close() }
}

func (c *Client) DeclareAll() error {
	if c == nil || c.ch == nil { return fmt.Errorf("nil channel") }
	if err := c.ch.ExchangeDeclare("orders_topic", "topic", true, false, false, false, nil); err != nil { return err }
	if err := c.ch.ExchangeDeclare("notifications_fanout", "fanout", true, false, false, false, nil); err != nil { return err }
	if err := c.ch.ExchangeDeclare("dlx", "direct", true, false, false, false, nil); err != nil { return err }
	_, err := c.ch.QueueDeclare("kitchen.q", true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "dlx",
		"x-dead-letter-routing-key": "dlq",
	})
	if err != nil { return err }
	if _, err = c.ch.QueueDeclare("notifications.q", true, false, false, false, nil); err != nil { return err }
	if _, err = c.ch.QueueDeclare("dlq", true, false, false, false, nil); err != nil { return err }
	if err := c.ch.QueueBind("kitchen.q", "kitchen.*.*", "orders_topic", false, nil); err != nil { return err }
	if err := c.ch.QueueBind("notifications.q", "", "notifications_fanout", false, nil); err != nil { return err }
	return nil
}

func (c *Client) PublishPersistent(exchange, key string, priority uint8, body []byte) error {
	return c.ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Priority:     priority,
		Timestamp:    time.Now().UTC(),
		ContentType:  "application/json",
		Body:         body,
	})
}

func (c *Client) Consume(queue, consumer string, prefetch int) (<-chan amqp.Delivery, error) {
	if err := c.ch.Qos(prefetch, 0, false); err != nil { return nil, err }
	return c.ch.Consume(queue, consumer, false, false, false, false, nil)
}
