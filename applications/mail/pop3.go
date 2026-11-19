package mail

import (
	"context"
	"errors"
	"time"

	"github.com/knadh/go-pop3"
	"horizon/foundation/system/governance"
)

func pop3Connect(ctx context.Context, host string, port int, username, password string, allowed []string) (*pop3.Conn, error) {
	if !allowedEndpoint(host, port, allowed) {
		return nil, governance.Problem("forbidden")
	}
	client := pop3.New(pop3.Opt{Host: host, Port: port, TLSEnabled: true, DialTimeout: 10 * time.Second})
	connection, err := client.NewConn()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*pop3.Conn, error) {
		_ = connection.Quit()
		return nil, err
	}
	if username != "" {
		if err = connection.Auth(username, password); err != nil {
			return fail(err)
		}
	} else {
		if err = connection.Auth("", password); err == nil {
			return fail(errors.New("pop3_username_required"))
		}
		return fail(governance.Problem("invalid"))
	}
	if _, _, err = connection.Stat(); err != nil {
		return fail(err)
	}
	return connection, nil
}

func testPOP3(ctx context.Context, host string, port int, username, password string, allowed []string) error {
	connection, err := pop3Connect(ctx, host, port, username, password, allowed)
	if err != nil {
		return err
	}
	if err = connection.Noop(); err != nil {
		_ = connection.Quit()
		return err
	}
	return connection.Quit()
}
