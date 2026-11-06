package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StreamName = "launchbase:jobs"
	GroupName  = "workers"
)

// Stream is the Redis stream the api publishes and the worker reads.
type Stream struct {
	rdb   *redis.Client
	Name  string
	Group string
	Dead  string
}

func Open(url string) (*Stream, error) {
	return OpenNamed(url, StreamName, GroupName)
}

func OpenNamed(url, name, group string) (*Stream, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = StreamName
	}
	if group == "" {
		group = GroupName
	}
	return &Stream{
		rdb:   redis.NewClient(opt),
		Name:  name,
		Group: group,
		Dead:  name + ":dead",
	}, nil
}

func (s *Stream) Close() error {
	return s.rdb.Close()
}

func (s *Stream) Ping(ctx context.Context) error {
	return s.rdb.Ping(ctx).Err()
}

func (s *Stream) EnsureGroup(ctx context.Context) error {
	err := s.rdb.XGroupCreateMkStream(ctx, s.Name, s.Group, "0").Err()
	if err != nil && !isBusyGroup(err) {
		return err
	}
	return nil
}

func (s *Stream) Publish(ctx context.Context, payload []byte) error {
	return s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: s.Name,
		Values: map[string]any{"payload": string(payload)},
	}).Err()
}

type Message struct {
	ID  string
	Job Job
	Raw []byte
}

func (s *Stream) Read(ctx context.Context, consumer string, block time.Duration) (Message, error) {
	res, err := s.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    s.Group,
		Consumer: consumer,
		Streams:  []string{s.Name, ">"},
		Count:    1,
		Block:    block,
	}).Result()
	if err != nil {
		return Message{}, err
	}
	if len(res) == 0 || len(res[0].Messages) == 0 {
		return Message{}, redis.Nil
	}
	msg, err := decodeMessage(res[0].Messages[0])
	if err != nil && msg.ID == "" {
		return Message{}, err
	}
	return msg, nil
}

// Claim returns messages that were delivered to a consumer and then left unacked.
func (s *Stream) Claim(ctx context.Context, consumer string, minIdle time.Duration) ([]Message, error) {
	msgs, _, err := s.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   s.Name,
		Group:    s.Group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    8,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(msgs))
	for _, msg := range msgs {
		decoded, err := decodeMessage(msg)
		if err != nil && decoded.ID == "" {
			return nil, err
		}
		out = append(out, decoded)
	}
	return out, nil
}

func (s *Stream) Ack(ctx context.Context, id string) error {
	return s.rdb.XAck(ctx, s.Name, s.Group, id).Err()
}

func (s *Stream) DeadLetter(ctx context.Context, raw []byte, reason string) error {
	return s.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: s.Dead,
		Values: map[string]any{
			"payload": string(raw),
			"error":   reason,
		},
	}).Err()
}

func (s *Stream) Pending(ctx context.Context) (int64, error) {
	n, err := s.rdb.XPending(ctx, s.Name, s.Group).Result()
	if err != nil {
		return 0, err
	}
	return n.Count, nil
}

func (s *Stream) DeadCount(ctx context.Context) (int64, error) {
	n, err := s.rdb.XLen(ctx, s.Dead).Result()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Stream) Delete(ctx context.Context) {
	_ = s.rdb.Del(ctx, s.Name, s.Dead).Err()
}

func decodeMessage(msg redis.XMessage) (Message, error) {
	raw, _ := msg.Values["payload"].(string)
	if raw == "" {
		return Message{ID: msg.ID}, fmt.Errorf("job payload is empty")
	}
	job, err := Decode([]byte(raw))
	if err != nil {
		return Message{ID: msg.ID, Raw: []byte(raw)}, err
	}
	return Message{ID: msg.ID, Job: job, Raw: []byte(raw)}, nil
}

func isBusyGroup(err error) bool {
	return err != nil && strings.Contains(err.Error(), "BUSYGROUP")
}
