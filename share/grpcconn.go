package share

import (
	"bytes"
	"net"
	"sync"
	"time"
	"zimnyaa/grpcssh/grpctun" // Import your generated proto package
)

type SendRecvGRPC interface {
    Send(m *grpctun.TunnelData) error
    Recv() (*grpctun.TunnelData, error)
}

type GrpcConn struct {
	stream SendRecvGRPC
	rbuf   *bytes.Buffer
	wbuf   *bytes.Buffer
	mu     sync.Mutex
}

func NewGrpcServerConn(stream grpctun.TunnelService_TunnelServer) *GrpcConn {
	return &GrpcConn{
		stream: stream,
		rbuf:   &bytes.Buffer{},
		wbuf:   &bytes.Buffer{},
	}
}

func NewGrpcClientConn(stream grpctun.TunnelService_TunnelClient) *GrpcConn {
	return &GrpcConn{
		stream: stream,
		rbuf:   &bytes.Buffer{},
		wbuf:   &bytes.Buffer{},
	}
}

func (c *GrpcConn) Read(b []byte) (n int, err error) {
	for c.rbuf.Len() == 0 {
		in, err := c.stream.Recv()
		if err != nil {
			return 0, err
		}
		c.rbuf.Write([]byte(in.GetData()))
	}
	return c.rbuf.Read(b)
}

func (c *GrpcConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wbuf.Write(b)

	if err := c.flush(); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *GrpcConn) flush() error {
	if err := c.stream.Send(&grpctun.TunnelData{Data: c.wbuf.Bytes()}); err != nil {
		return err
	}
	c.wbuf.Reset()
	return nil
}

func (c *GrpcConn) Close() error {
	return nil
}

func (c *GrpcConn) LocalAddr() net.Addr {
	return nil
}

func (c *GrpcConn) RemoteAddr() net.Addr {
	return nil
}

func (c *GrpcConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *GrpcConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *GrpcConn) SetWriteDeadline(t time.Time) error {
	return nil
}
