package notify

import "context"

func Run(ctx context.Context) error {
	// TODO: подписка на notifications_fanout -> notifications.q
	<-ctx.Done()
	return nil
}
