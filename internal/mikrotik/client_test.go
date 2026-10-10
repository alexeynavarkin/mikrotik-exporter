package mikrotik

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/go-routeros/routeros/v3"
	"github.com/go-routeros/routeros/v3/proto"
)

func deviceError(word, message string) error {
	return fmt.Errorf("wrapped: %w", &routeros.DeviceError{Sentence: &proto.Sentence{
		Word: word,
		Map:  map[string]string{"message": message},
	}})
}

func TestIsDeviceError(t *testing.T) {
	if !IsDeviceError(deviceError("!trap", "failure")) {
		t.Error("!trap must be a device error")
	}
	if IsDeviceError(deviceError("!fatal", "session terminated")) {
		t.Error("!fatal closes the connection and must not be a device error")
	}
	if IsDeviceError(io.EOF) || IsDeviceError(errors.New("boom")) {
		t.Error("transport errors must not be device errors")
	}
}

func TestIsUnsupportedCommand(t *testing.T) {
	if !IsUnsupportedCommand(deviceError("!trap", "no such command prefix")) {
		t.Error("expected unsupported command")
	}
	if IsUnsupportedCommand(deviceError("!trap", "failure")) || IsUnsupportedCommand(io.EOF) {
		t.Error("unexpected unsupported command")
	}
}
