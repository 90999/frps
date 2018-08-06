// Copyright 2016 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package net

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/KunTengRom/xfrps/utils/log"
)

//TCP监听结构
type TcpListener struct {
	net.Addr
	listener  net.Listener
	accept    chan Conn
	closeFlag bool
	log.Logger
}

//服务监听端口
func ListenTcp(bindAddr string, bindPort int64) (l *TcpListener, err error) {
	//解析地址
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", bindAddr, bindPort))
	if err != nil {
		return l, err
	}
	//bind地址,监听
	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return l, err
	}

	//服务listener
	l = &TcpListener{
		Addr:      listener.Addr(), //bind地址
		listener:  listener,        //listener结构
		accept:    make(chan Conn),
		closeFlag: false,
		Logger:    log.NewPrefixLogger(""),
	}

	// go 协程
	go func() {

		//dead loop
		for {
			//listen,返回connection
			conn, err := listener.AcceptTCP()
			if err != nil {
				if l.closeFlag {
					close(l.accept) //关闭accept chan
					return
				}
				continue
			}

			//新来一个连接，新建TCP connection
			c := NewTcpConn(conn)

			//新连接导入accept-channel
			l.accept <- c
		}
	}()
	return l, err
}

// Wait util get one new connection or listener is closed
// if listener is closed, err returned.
// accept 通过chan传过来， Accept仅供测试
func (l *TcpListener) Accept() (Conn, error) {
	conn, ok := <-l.accept
	if !ok {
		return conn, fmt.Errorf("channel for tcp listener closed")
	}
	return conn, nil
}

//关闭listener，仅供测试
func (l *TcpListener) Close() error {
	if !l.closeFlag {
		l.closeFlag = true
		l.listener.Close()
	}
	return nil
}

// Wrap for TCPConn.
type TcpConn struct {
	net.Conn
	log.Logger
}

//新建connection 结构, 包含Conn
func NewTcpConn(conn *net.TCPConn) (c *TcpConn) {
	c = &TcpConn{
		Conn:   conn,
		Logger: log.NewPrefixLogger(""),
	}
	return
}

//连接TCP server， 被HTTP代理调用
func ConnectTcpServer(addr string) (c Conn, err error) {
	servertAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}
	conn, err := net.DialTCP("tcp", nil, servertAddr)
	if err != nil {
		return
	}
	c = NewTcpConn(conn)
	return
}

// 通过HTTPproxy连接TCPServer
// ConnectTcpServerByHttpProxy try to connect remote server by http proxy.
// If httpProxy is empty, it will connect server directly.
func ConnectTcpServerByHttpProxy(httpProxy string, serverAddr string) (c Conn, err error) {

	//如果传入的httpproxy为空,则直接连接TcpServer
	if httpProxy == "" {
		return ConnectTcpServer(serverAddr)
	}

	//解析http proxy url
	var proxyUrl *url.URL
	if proxyUrl, err = url.Parse(httpProxy); err != nil {
		return
	}

	var proxyAuth string
	if proxyUrl.User != nil {
		username := proxyUrl.User.Username()
		passwd, _ := proxyUrl.User.Password()
		proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+passwd))
	}

	if proxyUrl.Scheme != "http" {
		err = fmt.Errorf("Proxy URL scheme must be http, not [%s]", proxyUrl.Scheme)
		return
	}

	//连接proxy Host
	if c, err = ConnectTcpServer(proxyUrl.Host); err != nil {
		return
	}

	//构建HTTP CONNECT消息到代理地址
	req, err := http.NewRequest("CONNECT", "http://"+serverAddr, nil)
	if err != nil {
		return
	}
	//设置http auth 头
	if proxyAuth != "" {
		req.Header.Set("Proxy-Authorization", proxyAuth)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	//发送请求
	req.Write(c)

	//读取请求
	resp, err := http.ReadResponse(bufio.NewReader(c), req)
	if err != nil {
		return
	}
	resp.Body.Close()

	//检查连接是否成功
	if resp.StatusCode != 200 {
		err = fmt.Errorf("ConnectTcpServer using proxy error, StatusCode [%d]", resp.StatusCode)
		return
	}

	return
}
