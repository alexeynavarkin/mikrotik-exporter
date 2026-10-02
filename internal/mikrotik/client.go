package mikrotik

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
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
	// PlainText disables TLS (api service, port 8728) instead of api-ssl (port 8729).
	PlainText bool
}

type RetryClient struct {
	cfg RetryClientConfig

	lg *zap.Logger

	client     *routeros.Client
	clientLock sync.Mutex
}

func NewRetryClient(cfg RetryClientConfig, lg *zap.Logger) Client {
	return &RetryClient{
		cfg: cfg,
		lg:  lg,
	}
}

func (c *RetryClient) connect(ctx context.Context) error {
	var (
		client *routeros.Client
		err    error
	)
	if c.cfg.PlainText {
		client, err = routeros.DialContext(ctx, c.cfg.Address, c.cfg.Username, c.cfg.Password)
	} else {
		client, err = routeros.DialTLSContext(
			ctx,
			c.cfg.Address,
			c.cfg.Username,
			c.cfg.Password,
			&tls.Config{
				InsecureSkipVerify: true,
			},
		)
	}
	if err != nil {
		return err
	}

	// In sync mode the library ignores the context passed to RunContext,
	// so a stuck device would block the scrape forever. Async mode honors it.
	client.Async()
	c.client = client

	return nil
}

func (c *RetryClient) disconnect() {
	if c.client == nil {
		return
	}

	if err := c.client.Close(); err != nil {
		c.lg.Debug("failed to close client", zap.String("address", c.cfg.Address), zap.Error(err))
	}
	c.client = nil
}

func (c *RetryClient) RunContext(ctx context.Context, sentences ...string) (*routeros.Reply, error) {
	c.clientLock.Lock()
	defer c.clientLock.Unlock()

	var err error
	for i := 0; i < c.cfg.RetryCount+1; i++ {
		if c.client == nil {
			if err = c.connect(ctx); err != nil {
				return nil, fmt.Errorf("failed to connect: %w", err)
			}
		}

		var res *routeros.Reply
		res, err = c.client.RunContext(ctx, sentences...)
		if err == nil {
			return res, nil
		}

		if IsDeviceError(err) {
			// The device answered with !trap, the connection itself is fine.
			return nil, err
		}

		// Any other error (EOF, broken pipe, reset, timeout, !fatal) leaves the
		// connection in an unknown state, so drop it and reconnect.
		c.lg.Warn(
			"command failed, reconnecting",
			zap.String("address", c.cfg.Address),
			zap.Strings("command", sentences),
			zap.Error(err),
		)
		c.disconnect()

		if ctx.Err() != nil {
			break
		}
	}

	return nil, err
}

// IsDeviceError reports whether err is a !trap reply from the device,
// meaning the command failed but the connection is still usable.
func IsDeviceError(err error) bool {
	var devErr *routeros.DeviceError
	if !errors.As(err, &devErr) {
		return false
	}

	return devErr.Sentence == nil || devErr.Sentence.Word != "!fatal"
}

// IsUnsupportedCommand reports whether the device rejected the command because
// it does not exist (e.g. /container without the container package).
func IsUnsupportedCommand(err error) bool {
	var devErr *routeros.DeviceError
	if !errors.As(err, &devErr) || devErr.Sentence == nil {
		return false
	}

	return strings.Contains(devErr.Sentence.Map["message"], "no such command")
}
