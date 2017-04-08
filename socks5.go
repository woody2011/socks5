package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
)

var (
	errVersion          = errors.New("socks5 version number error")
	errMethodNum        = errors.New("socks5 method num error")
	errMethodNotSupport = errors.New("socks5 method not support")
	errCommand          = errors.New("socks5 cmd not supported")
	errProto            = errors.New("socks protocal error")
)

func handshake(c net.Conn) (err error) {
	buf := make([]byte, 255+1+1)
	if _, err = io.ReadFull(c, buf[:2]); err != nil {
		fmt.Printf("ReadFull error: %s\n", err.Error())
		return
	}
	// check
	if buf[0] != 0x5 {
		fmt.Printf("version=%d, not supported\n", int(buf[0]))
		err = errVersion
		return
	}
	if buf[1] == 0 {
		err = errMethodNum
		return
	}
	methodlen := int(buf[1])
	if _, err = io.ReadFull(c, buf[2:2+methodlen]); err != nil {
		fmt.Printf("ReadFull error: %s", err.Error())
		return
	}
	methodflag := false
	for _, v := range buf[2 : 2+methodlen] {
		// only support METHOD = 0
		if v == 0 {
			methodflag = true
			break
		}
	}
	if !methodflag {
		err = errMethodNotSupport
		return
	}
	return
}

func parseRequest(c net.Conn) (host string, err error) {
	buf := make([]byte, 4+1+255+2)
	if _, err = io.ReadFull(c, buf[:4+1]); err != nil {
		fmt.Printf("ReadFull error: %s", err.Error())
		return
	}

	if buf[0] != 0x5 {
		fmt.Printf("version error, version:%d\n", int(buf[0]))
		err = errVersion
		return
	}
	if buf[1] != 0x1 {
		fmt.Printf("cmd=%d not supported\n", int(buf[1]))
		err = errCommand
		return
	}
	if buf[2] != 0x0 {
		err = errProto
		return
	}
	var reqlen int
	switch buf[3] {
	case 0x1:
		reqlen = 4 + 4 + 2
	case 0x3:
		reqlen = 4 + int(buf[4]) + 2
	case 0x4:
		reqlen = 4 + 16 + 2
	default:
		fmt.Printf("addr type[%d] error\n", int(buf[3]))
		err = errCommand
		return
	}
	if _, err = io.ReadFull(c, buf[4+1:reqlen]); err != nil {
		fmt.Printf("ReadFull error: %s\n", err.Error())
		return
	}
	addr := buf[3:reqlen]
	addrlen := len(addr)
	port := (int(addr[addrlen-2]) << 8) + int(addr[addrlen-1])
	switch buf[3] {
	case 0x1:
		host = net.IPv4(addr[1], addr[2], addr[3], addr[4]).String() + ":" + strconv.Itoa(port)
	case 0x3:
		host = string(addr[2:int(addr[1])]) + ":" + strconv.Itoa(port)
	case 0x4:
		host = net.IP(addr[1:17]).String() + ":" + strconv.Itoa(port)
	}
	return
}

func HandleConn(c net.Conn) {
	defer c.Close()
	var err error
	if err = handshake(c); err != nil {
		fmt.Printf("handshake error: %s\n", err.Error())
		return
	}
	if _, err = c.Write([]byte{0x5, 0x0}); err != nil {
		fmt.Printf("Write error: %s", err.Error())
		return
	}
	var host string
	if host, err = parseRequest(c); err != nil {
		fmt.Printf("parseRequest error: %s\n", err.Error())
		return
	}

	// reply
	if _, err = c.Write([]byte{0x5, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}); err != nil {
		fmt.Printf("Write error: %s\n", err.Error())
		return
	}

	var remote net.Conn
	// connect to remote
	if remote, err = connRemote(host); err != nil {
		fmt.Printf("connRemote error: %s\n", err.Error())
		return
	}

	go doWork(remote, c)
	doWork(c, remote)
	//fmt.Printf("conn close\n")
}

func doWork(dest, src net.Conn) {
	buf := make([]byte, 1024)
	var n int
	var err error
	for {
		if n, err = src.Read(buf); err != nil {
			break
		}
		if n <= 0 {
			break
		}
		if _, err = dest.Write(buf[:n]); err != nil {
			break
		}
	}
	//fmt.Printf("read addr:%s, write addr:%s\n", dest.RemoteAddr(), src.RemoteAddr())
}

func connRemote(host string) (c net.Conn, err error) {
	//fmt.Printf("connect to remote addr: %s\n", host)
	c, err = net.Dial("tcp", host)
	return
}

func init() {
	flag.IntVar(&listen_port, "port", 9000, "listen port")
	flag.Parse()
}

var listen_port int

func main() {
	host := ":" + strconv.Itoa(listen_port)
	ln, err := net.Listen("tcp", host)
	if err != nil {
		fmt.Printf("Listen error: %s\n", err.Error())
		return
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Accept error: %s\n", err.Error())
			continue
		}
		go HandleConn(conn)
	}
}
