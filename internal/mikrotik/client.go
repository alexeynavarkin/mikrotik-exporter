package mikrotik

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/go-routeros/routeros/v3"
	"go.uber.org/zap"
)

type Client interface {
	RunContext(ctx context.Context, sentences ...string) (*routeros.Reply, error)
}

type RetryClientConfig struct {
	Username   string
	Password   string
	Address    string
	RetryCount int
}

type RetryClient struct {
	cfg RetryClientConfig

	lg *zap.Logger

	client     *routeros.Client
	clientLock sync.Mutex
}

func NewRetryClient(cfg RetryClientConfig, lg *zap.Logger) Client {
	return &RetryClient{
		cfg:        cfg,
		lg:         lg,
		client:     nil,
		clientLock: sync.Mutex{},
	}
}

func (c *RetryClient) lazyInit(ctx context.Context) error {
	if c.client != nil {
		return nil
	}

	return c.init(ctx)
}

func (c *RetryClient) init(ctx context.Context) error {
	c.clientLock.Lock()
	defer c.clientLock.Unlock()

	if c.client != nil {
		err := c.client.Close()
		if err != nil {
			c.lg.Error("failed to close client", zap.Error(err))
		}
	}

	var err error
	c.client, err = routeros.DialTLSContext(
		ctx,
		c.cfg.Address,
		c.cfg.Username,
		c.cfg.Password,
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)

	return err
}

func (c *RetryClient) RunContext(ctx context.Context, sentences ...string) (*routeros.Reply, error) {
	err := c.lazyInit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to init client: %w", err)
	}

	var res *routeros.Reply
	for i := 0; i < c.cfg.RetryCount+1; i++ {
		res, err = c.client.RunContext(ctx, sentences...)
		if err == nil {
			return res, nil
		}

		if errors.Is(err, io.ErrClosedPipe) {
			c.lg.Warn("command failed, re init client", zap.Error(err))
			err := c.init(ctx)
			if err != nil {
				return nil, err
			}

			continue
		}

		break
	}

	return res, err
}
