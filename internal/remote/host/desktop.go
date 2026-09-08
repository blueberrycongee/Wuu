package host

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"time"
)

// DesktopAppServer forwards protocol lines to a local execution service.
// The secret authenticates this process; paired-device authentication remains
// on the encrypted remote connection. No task is owned by this transport.
func DesktopAppServer(address, token string) (func(context.Context, io.Reader, io.Writer) error, error) {
	name, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(name) == nil || !net.ParseIP(name).IsLoopback() || token == "" {
		return nil, errors.New("desktop app-server requires a loopback address and token")
	}
	return func(ctx context.Context, in io.Reader, out io.Writer) error {
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
		if err != nil {
			return err
		}
		defer conn.Close()
		stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
		defer stop()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err := json.NewEncoder(conn).Encode(map[string]string{"token": token}); err != nil {
			return err
		}
		reader := bufio.NewReader(conn)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return err
		}
		var ack struct {
			Ready bool `json:"ready"`
		}
		if json.Unmarshal(line, &ack) != nil || !ack.Ready {
			return errors.New("desktop app-server rejected connection")
		}
		_ = conn.SetDeadline(time.Time{})
		go func() { _, _ = io.Copy(conn, in); _ = conn.Close() }()
		_, err = io.Copy(out, reader)
		return err
	}, nil
}
