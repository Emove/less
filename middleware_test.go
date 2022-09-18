package less

import (
	"context"
	"testing"
)

func TestChain(t *testing.T) {

	h := func(ctx context.Context, ch Channel, message interface{}) error {
		t.Logf("handle message: %v", message)
		return nil
	}

	err := Chain(middleware1(t), middleware2(t), middleware3(t))(h)(context.Background(), nil, "middleware test")
	if err != nil {
		t.Errorf("expect %v, got %v", nil, err)
	}
}

func middleware1(t *testing.T) Middleware {
	return func(handler Handler) Handler {
		return func(ctx context.Context, ch Channel, message interface{}) error {
			t.Logf("middleware1 before...")

			err := handler(ctx, ch, message)

			t.Logf("middleware1 after...")
			return err
		}
	}
}

func middleware2(t *testing.T) Middleware {
	return func(handler Handler) Handler {
		return func(ctx context.Context, ch Channel, message interface{}) error {
			t.Logf("middleware2 before...")

			err := handler(ctx, ch, message)

			t.Logf("middleware2 after...")
			return err
		}
	}
}

func middleware3(t *testing.T) Middleware {
	return func(handler Handler) Handler {
		return func(ctx context.Context, ch Channel, message interface{}) error {
			t.Logf("middleware3 before...")

			err := handler(ctx, ch, message)

			t.Logf("middleware3 after...")
			return err
		}
	}
}
