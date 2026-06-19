package main

import (
	"context"
	"net"
	"net/url"

	"golang.org/x/net/proxy"
)

func dialContextForProxy(proxyURL string) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if proxyURL == "" {
		return nil, nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "socks5" && u.Scheme != "socks5h" {
		return nil, nil
	}
	addr := u.Host
	if u.Scheme == "socks5h" && u.Host == "" {
		addr = u.Host
	}
	var auth *proxy.Auth
	if u.User != nil {
		pw, _ := u.User.Password()
		auth = &proxy.Auth{User: u.User.Username(), Password: pw}
	}
	d, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return d.Dial(network, addr)
	}, nil
}

func isSOCKS5(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	return p.Scheme == "socks5" || p.Scheme == "socks5h"
}
