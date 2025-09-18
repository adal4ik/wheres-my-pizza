package rabbitmq

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string // default "/"
	UseTLS   bool   // optional
}

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel

	acks <-chan amqp.Confirmation // для publisher confirms
	mu   sync.Mutex               // сериализуем Publish при использовании confirms
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

func Dial(cfg Config) (*Client, error) {
	if cfg.VHost == "" {
		cfg.VHost = "/"
	}
	scheme := "amqp"
	if cfg.UseTLS {
		scheme = "amqps"
	}
	url := fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		scheme, cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.VHost)

	var (
		conn *amqp.Connection
		err  error
	)
	if cfg.UseTLS {
		conn, err = amqp.DialTLS(url, &tls.Config{MinVersion: tls.VersionTLS12})
	} else {
		conn, err = amqp.Dial(url)
	}
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	// Включаем publisher confirms и подписываемся на подтверждения
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	acks := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	return &Client{conn: conn, ch: ch, acks: acks}, nil
}

// Лёгкая health-проверка соединения
func (c *Client) Ping() error {
	if c.conn == nil || c.conn.IsClosed() {
		return errors.New("rabbitmq connection is closed")
	}
	return nil
}

// Publish публикует сообщение и ждёт ack/nack от брокера.
// Не вызывает горутинно одновременно (сериализуется mutex-ом).
func (c *Client) Publish(ctx context.Context, exchange, key string,
	body []byte, headers amqp.Table, contentType string, persistent bool) error {

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

	// ждём publisher confirm или отмену контекста
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
