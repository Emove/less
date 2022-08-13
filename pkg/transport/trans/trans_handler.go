package trans

import (
	"github.com/bytedance/gopkg/util/gopool"
	"less/internal/handler"
	"less/pkg/proto"
	"less/pkg/transport"
	"net"
)

func NewTransportHandler(opts *transport.TransServerOption, conn transport.Connection, handler *handler.PipelineHandler) transport.TransHandler {
	return &transportHandler{
		opts:    opts,
		conn:    conn,
		handler: handler,
		proto:   proto.GenericProtocol,
	}
}

type transportHandler struct {
	conn    transport.Connection
	opts    *transport.TransServerOption
	handler *handler.PipelineHandler
	proto   proto.Protocol
}

func (tsp *transportHandler) OnRequest() error {
	r := tsp.conn.Reader()
	// unpack
	header, err := r.Next(int(tsp.proto.HeaderLength()))
	if err != nil {
		return err
	}

	// read bodySize
	bodySize := tsp.proto.ByteOrder().Uint32(header)
	if err != nil {
		return err
	}

	// decorate a reader
	reader := newLimitReader(r, bodySize)
	defer reader.Release()

	// decode body
	codec := tsp.opts.Codec
	req, err := codec.Decode(reader)
	if err != nil {
		return err
	}

	gopool.Go(func() {
		msg := transport.NewMessage(transport.MessageTypeInbound, req)
		ctx := transport.NewContext(tsp, msg)

		if tsp.handler != nil {
			if err = tsp.handler.Handle(ctx); err != nil {
				// TODO log
				return
			}
		}

		tsp.opts.OnMessage(ctx)
	})

	return nil
}

func (tsp *transportHandler) Remote() net.Addr {
	return tsp.conn.RemoteAddr()
}

func (tsp *transportHandler) Local() net.Addr {
	return tsp.conn.LocalAddr()
}

func (tsp *transportHandler) Send(data interface{}) error {

	msg := transport.NewMessage(transport.MessageTypeOutbound, data)
	ctx := transport.NewContext(tsp, msg)

	err := tsp.handler.Handle(ctx)
	if err != nil {
		return err
	}

	w := tsp.conn.Writer()

	err = tsp.opts.Codec.Encode(data, w)
	if err != nil {
		w.Release()
		return err
	}
	err = w.Flush()
	w.Release()
	return err
}

var _ transport.Reader = (*limitReader)(nil)

type limitReader struct {
	remain    uint32
	decorator transport.Reader
}

func newLimitReader(decorator transport.Reader, limit uint32) transport.Reader {
	return &limitReader{
		decorator: decorator,
		remain:    limit,
	}
}

func (lr *limitReader) Next(n int) (buf []byte, err error) {
	if int(lr.remain)-n < 0 {
		// TODO return an explicit error
		return buf, err
	}
	if buf, err = lr.decorator.Next(n); err != nil {
		return
	}
	lr.remain -= uint32(n)
	return
}

func (lr *limitReader) Peek(n int) (buf []byte, err error) {
	if int(lr.remain)-n < 0 {
		// TODO return an explicit error
		return buf, err
	}
	return lr.decorator.Peek(n)
}

func (lr *limitReader) Skip(n int) (err error) {
	if int(lr.remain)-n < 0 {
		// TODO return an explicit error
		return err
	}
	if err = lr.decorator.Skip(n); err != nil {
		return
	}
	lr.remain -= uint32(n)
	return
}

func (lr *limitReader) Release() {
	if lr.remain != 0 {
		_ = lr.decorator.Skip(int(lr.remain))
	}
	lr.decorator.Release()
	lr.decorator = nil
}
