package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/examples/device-gateway/protocol"
)

var (
	errSessionMissing  = errors.New("session missing")
	errUnauthenticated = errors.New("device not authenticated")
	errInvalidSecret   = errors.New("invalid device secret")
	errDuplicateAuth   = errors.New("device already authenticated")
)

func onChannel(gw *gateway) less.OnChannel {
	return func(ctx context.Context, ch less.Channel) (context.Context, error) {
		gw.registry.register(ch)
		return ctx, nil
	}
}

func onChannelClosed(gw *gateway) less.OnChannelClosed {
	return func(_ context.Context, ch less.Channel, _ error) {
		gw.registry.remove(ch)
	}
}

func newRouter(gw *gateway) less.Router {
	return func(_ context.Context, _ less.Channel, msg interface{}) (less.Handler, error) {
		switch msg.(type) {
		case *protocol.AuthMessage:
			return authHandler(gw), nil
		case *protocol.HeartbeatMessage:
			return heartbeatHandler(gw), nil
		case *protocol.TelemetryMessage:
			return telemetryHandler(gw), nil
		case *protocol.CommandAckMessage:
			return commandAckHandler(gw), nil
		default:
			return nil, fmt.Errorf("unsupported message type %T", msg)
		}
	}
}

func authHandler(gw *gateway) less.Handler {
	return func(_ context.Context, ch less.Channel, msg interface{}) error {
		auth, ok := msg.(*protocol.AuthMessage)
		if !ok {
			return fmt.Errorf("unexpected auth message type %T", msg)
		}

		sess, ok := gw.registry.session(ch)
		if !ok {
			return errSessionMissing
		}
		if sess.authenticated {
			return errDuplicateAuth
		}
		if auth.Secret != demoSecret {
			return errInvalidSecret
		}
		if err := ch.Write(protocol.AuthAck("ok")); err != nil {
			return err
		}
		if !gw.registry.authenticate(ch, auth.DeviceID) {
			return errSessionMissing
		}

		return nil
	}
}

func heartbeatHandler(gw *gateway) less.Handler {
	return func(_ context.Context, ch less.Channel, msg interface{}) error {
		if _, ok := msg.(*protocol.HeartbeatMessage); !ok {
			return fmt.Errorf("unexpected heartbeat message type %T", msg)
		}
		if !gw.registry.heartbeat(ch, time.Now()) {
			return errUnauthenticated
		}

		return nil
	}
}

func telemetryHandler(gw *gateway) less.Handler {
	return func(_ context.Context, ch less.Channel, msg interface{}) error {
		if _, ok := msg.(*protocol.TelemetryMessage); !ok {
			return fmt.Errorf("unexpected telemetry message type %T", msg)
		}

		sess, ok := gw.registry.session(ch)
		if !ok {
			return errSessionMissing
		}
		if !sess.authenticated {
			return errUnauthenticated
		}

		requestID := gw.commandSeq.Add(1)
		return ch.Write(protocol.Command(requestID, "reboot"))
	}
}

func commandAckHandler(gw *gateway) less.Handler {
	return func(_ context.Context, ch less.Channel, msg interface{}) error {
		if _, ok := msg.(*protocol.CommandAckMessage); !ok {
			return fmt.Errorf("unexpected command ack message type %T", msg)
		}

		sess, ok := gw.registry.session(ch)
		if !ok {
			return errSessionMissing
		}
		if !sess.authenticated {
			return errUnauthenticated
		}

		return nil
	}
}
