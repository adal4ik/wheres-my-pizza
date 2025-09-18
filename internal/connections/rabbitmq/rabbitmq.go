package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"wheres-my-pizza/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel

	acks <-chan amqp.Confirmation
	mu   sync.Mutex
}

func (c *Client) Channel() *amqp.Channel { return c.ch }

func (c *Client) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func Dial(cfg config.RabbitMQConfig) (*Client, error) {
	if cfg.VHost == "" {
		cfg.VHost = "/"
	}
	scheme := "amqp"
	url := fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		scheme, cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.VHost)

	var (
		conn *amqp.Connection
		err  error
	)
	conn, err = amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	acks := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	return &Client{conn: conn, ch: ch, acks: acks}, nil
}

func (c *Client) Ping() error {
	if c.conn == nil || c.conn.IsClosed() {
		return errors.New("rabbitmq connection is closed")
	}
	return nil
}

func (c *Client) Publish(ctx context.Context, exchange, key string,
	body []byte, headers amqp.Table, contentType string, persistent bool,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	mode := amqp.Transient
	if persistent {
		mode = amqp.Persistent
	}

	if err := c.ch.PublishWithContext(
		ctx,
		exchange,
		key,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			DeliveryMode: mode,
			ContentType:  contentType,
			Timestamp:    time.Now(),
			Headers:      headers,
			Body:         body,
		},
	); err != nil {
		return err
	}

	select {
	case conf := <-c.acks:
		if conf.Ack {
			return nil
		}
		return errors.New("publish NACK from broker")
	case <-ctx.Done():
		return ctx.Err()
	}
}
