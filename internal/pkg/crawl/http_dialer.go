package crawl

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
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

func (crawl *Crawl) wrapConnection(c net.Conn, output io.Writer) net.Conn {
	respTempFile, err := ioutil.TempFile(filepath.Join(crawl.JobPath, "temp"), "*.temp")
	if err != nil {
		log.Fatal(err)
	}

	reqTempFile, err := ioutil.TempFile(filepath.Join(crawl.JobPath, "temp"), "*.temp")
	if err != nil {
		log.Fatal(err)
	}

	return &customConnection{
		Conn:   c,
		Reader: io.TeeReader(c, respTempFile),
		Writer: io.MultiWriter(reqTempFile, c),
	}
}

func (dialer *customDialer) CustomDial(network, address string) (net.Conn, error) {
	conn, err := dialer.Dial(network, address)
	if err != nil {
		return nil, err
	}

	file, err := os.Create("testeuh.txt")
	if err != nil {
		panic(err)
	}

	return dialer.c.wrapConnection(conn, file), nil
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

	file, err := os.Create("testeuh.txt")
	if err != nil {
		panic(err)
	}

	return dialer.c.wrapConnection(tlsConn, file), nil // return a wrapped net.Conn
}
