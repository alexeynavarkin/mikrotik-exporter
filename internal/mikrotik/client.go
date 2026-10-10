package mikrotik

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

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
	conn       net.Conn
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
		conn net.Conn
		err  error
	)
	if c.cfg.PlainText {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", c.cfg.Address)
	} else {
		conn, err = (&tls.Dialer{Config: &tls.Config{InsecureSkipVerify: true}}).DialContext(ctx, "tcp", c.cfg.Address)
	}
	if err != nil {
		return fmt.Errorf("could not connect to router os: %w", err)
	}

	client, err := routeros.NewClient(conn)
	if err != nil {
		_ = conn.Close()
		return err
	}

	err = withDeadline(ctx, conn, func() error {
		return client.LoginContext(ctx, c.cfg.Username, c.cfg.Password)
	})
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("could not login: %w", err)
	}

	c.client, c.conn = client, conn

	return nil
}

// withDeadline bounds fn by ctx through the connection deadline. The library
// ignores the context in sync mode, and its async mode loses replies that
// arrive before the request tag is registered, so neither can be relied on.
func withDeadline(ctx context.Context, conn net.Conn, fn func() error) error {
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})

	err := fn()

	if !stop() && ctx.Err() != nil && err != nil {
		err = fmt.Errorf("%w: %w", ctx.Err(), err)
	}
	_ = conn.SetDeadline(time.Time{})

	return err
}

func (c *RetryClient) disconnect() {
	if c.client == nil {
		return
	}

	if err := c.client.Close(); err != nil {
		c.lg.Debug("failed to close client", zap.String("address", c.cfg.Address), zap.Error(err))
	}
	c.client, c.conn = nil, nil
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
		err = withDeadline(ctx, c.conn, func() error {
			var runErr error
			res, runErr = c.client.RunContext(ctx, sentences...)
			return runErr
		})
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
