package queue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
	"github.com/redis/go-redis/v9"
)

// JobHandler runs one job. A nil error means the message was understood.
// Result.Busy leaves it pending so a crashed owner can be reclaimed.
type JobHandler func(context.Context, Job) (Result, error)

// Consumer publishes the outbox and reads the Redis stream.
type Consumer struct {
	Store  *store.DB
	Stream *Stream
	Handle JobHandler
	Name   string
	Idle   time.Duration
	Block  time.Duration
	Log    io.Writer
}

func (c *Consumer) Run(ctx context.Context) error {
	if c.Idle == 0 {
		c.Idle = 30 * time.Second
	}
	if c.Block == 0 {
		c.Block = 2 * time.Second
	}
	if err := c.Stream.EnsureGroup(ctx); err != nil {
		return err
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if _, err := c.Store.PublishPending(ctx, func(payload []byte) error {
			return c.Stream.Publish(ctx, payload)
		}); err != nil && ctx.Err() == nil {
			c.logf("publish: %v", err)
		}
		claimed, err := c.Stream.Claim(ctx, c.Name, c.Idle)
		if err != nil && ctx.Err() == nil {
			return err
		}
		if len(claimed) == 0 {
			msg, err := c.Stream.Read(ctx, c.Name, c.Block)
			if err != nil {
				if errors.Is(err, redis.Nil) || ctx.Err() != nil {
					continue
				}
				return err
			}
			claimed = []Message{msg}
		}
		for _, msg := range claimed {
			c.deliver(msg)
		}
	}
}

func (c *Consumer) deliver(msg Message) {
	// The job keeps running after the consumer is asked to stop.
	// The loop does not read another job until this one returns.
	if msg.Job.DeploymentID == "" {
		_ = c.Stream.DeadLetter(context.Background(), msg.Raw, "job payload is not valid")
		_ = c.Stream.Ack(context.Background(), msg.ID)
		return
	}
	res, err := c.Handle(context.Background(), msg.Job)
	if err != nil {
		c.logf("job %s: %v", msg.Job.DeploymentID, err)
		return
	}
	if res.Busy {
		return
	}
	if res.Dead {
		if err := c.Stream.DeadLetter(context.Background(), msg.Raw, res.Reason); err != nil {
			c.logf("dead letter: %v", err)
			return
		}
	}
	if err := c.Stream.Ack(context.Background(), msg.ID); err != nil {
		c.logf("ack: %v", err)
	}
}

func (c *Consumer) logf(format string, args ...any) {
	if c.Log == nil {
		return
	}
	fmt.Fprintf(c.Log, format+"\n", args...)
}
