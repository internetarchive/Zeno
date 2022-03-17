package crawl

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
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
}

func (cc *customConnection) Read(b []byte) (int, error) {
	return cc.Reader.Read(b)
}

func (cc *customConnection) Write(b []byte) (int, error) {
	return cc.Writer.Write(b)
}

func (crawl *Crawl) wrapConnection(c net.Conn, URLString string) net.Conn {
	respReader, respWriter := io.Pipe()
	reqReader, reqWriter := io.Pipe()

	URL, err := url.Parse(URLString)
	if err != nil {
		panic(err)
	}

	go crawl.writeWARCFromConnection(respReader, reqReader, URL)

	return &customConnection{
		Conn:   c,
		Reader: io.TeeReader(c, respWriter),
		Writer: io.MultiWriter(c, reqWriter),
	}
}

func (dialer *customDialer) CustomDial(network, address string) (net.Conn, error) {
	conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	return dialer.c.wrapConnection(conn, "http://"+address), nil
}

func (dialer *customDialer) CustomDialTLS(network, address string) (net.Conn, error) {
	plainConn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	cfg := new(tls.Config)

	u, err := url.Parse(fmt.Sprintf("https://%s", address))
	if err != nil {
		return nil, err
	}

	serverName := u.Host[:strings.LastIndex(u.Host, ":")]
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

	return dialer.c.wrapConnection(tlsConn, "https://"+address), nil // return a wrapped net.Conn
}
