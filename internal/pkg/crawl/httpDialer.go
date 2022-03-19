package crawl

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type customDialer struct {
	net.Dialer
	c *Crawl
}

type customConnection struct {
	net.Conn
	io.Reader
	io.Writer
	c []io.Closer
}

func (cc *customConnection) Read(b []byte) (int, error) {
	return cc.Reader.Read(b)
}

func (cc *customConnection) Write(b []byte) (int, error) {
	return cc.Writer.Write(b)
}

func (cc *customConnection) Close() error {
	for _, c := range cc.c {
		c.Close()
	}

	return cc.Conn.Close()
}

func (crawl *Crawl) wrapConnection(c net.Conn, URL *url.URL) net.Conn {
	reqReader, reqWriter := io.Pipe()
	respReader, respWriter := io.Pipe()

	crawl.WaitGroup.Add(1)
	go crawl.writeWARCFromConnection(reqReader, respReader, URL)

	return &customConnection{
		Conn:   c,
		c:      []io.Closer{reqWriter, respWriter},
		Reader: io.TeeReader(c, respWriter),
		Writer: io.MultiWriter(c, reqWriter),
	}
}

func (dialer *customDialer) DialRequest(req *http.Request) (net.Conn, error) {
	switch req.URL.Scheme {
	case "http":
		return dialer.CustomDial("tcp", req.Host+":80", req.URL)
	case "https":
		return dialer.CustomDialTLS("tcp", req.Host+":443", req.URL)
	default:
		panic("WTF?!?")
	}
}

func (dialer *customDialer) CustomDial_(network, address string) (net.Conn, error) {
	u, _ := url.Parse("http://" + address)
	return dialer.CustomDial(network, address, u)
}

func (dialer *customDialer) CustomDialTLS_(network, address string) (net.Conn, error) {
	u, _ := url.Parse("https://" + address)
	return dialer.CustomDialTLS(network, address, u)
}

func (dialer *customDialer) CustomDial(network, address string, URL *url.URL) (net.Conn, error) {
	conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	return dialer.c.wrapConnection(conn, URL), nil
}

func (dialer *customDialer) CustomDialTLS(network, address string, URL *url.URL) (net.Conn, error) {
	plainConn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	cfg := new(tls.Config)
	serverName := address[:strings.LastIndex(address, ":")]
	cfg.ServerName = serverName

	tlsConn := tls.Client(plainConn, cfg)

	errc := make(chan error, 2)
	timer := time.AfterFunc(time.Second, func() {
		errc <- errors.New("TLS handshake timeout")
	})

	go func() {
		err := tlsConn.Handshake()
		timer.Stop()
		errc <- err
	}()
	if err := <-errc; err != nil {
		plainConn.Close()
		return nil, err
	}

	if !cfg.InsecureSkipVerify {
		if err := tlsConn.VerifyHostname(cfg.ServerName); err != nil {
			plainConn.Close()
			return nil, err
		}
	}

	return dialer.c.wrapConnection(tlsConn, URL), nil // return a wrapped net.Conn
}
