package logs

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisHub publishes each event on launchbase:logs:<deploymentId>.
type RedisHub struct {
	rdb *redis.Client
}

func NewRedisHub(url string) (*RedisHub, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &RedisHub{rdb: redis.NewClient(opt)}, nil
}

func (h *RedisHub) Close() error {
	return h.rdb.Close()
}

func (h *RedisHub) Ping(ctx context.Context) error {
	return h.rdb.Ping(ctx).Err()
}

func channelName(deploymentID string) string {
	return "launchbase:logs:" + deploymentID
}

func (h *RedisHub) Publish(ctx context.Context, e Event) error {
	return h.rdb.Publish(ctx, channelName(e.DeploymentID), e.Marshal()).Err()
}

func (h *RedisHub) Subscribe(ctx context.Context, deploymentID string) (<-chan Event, func(), error) {
	pubsub := h.rdb.Subscribe(ctx, channelName(deploymentID))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, nil, err
	}
	out := make(chan Event, 64)
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			_ = pubsub.Close()
		})
	}
	go func() {
		defer close(out)
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				cancel()
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				e, err := Decode([]byte(msg.Payload))
				if err != nil {
					continue
				}
				select {
				case out <- e:
				case <-ctx.Done():
					cancel()
					return
				}
			}
		}
	}()
	return out, cancel, nil
}
