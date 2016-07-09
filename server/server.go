package server

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	bufSize = 4096

	addrTypeIPV4   = 0x01
	addrTypeDomain = 0x03
	addrTypeIPV6   = 0x04
)

var (
	errAddrType = errors.New("unsupport address type")
)

type Server struct {
	addr    *net.TCPAddr
	logger  *log.Logger
	pool    sync.Pool
	timeout time.Duration
}

func New(addr string, timeout time.Duration) (server *Server, err error) {
	a, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}

	server = &Server{
		addr:   a,
		logger: log.New(os.Stderr, "", log.LstdFlags),
		pool: sync.Pool{
			New: func() interface{} { return make([]byte, 4096) },
		},
		timeout: timeout,
	}
	return
}

func (s *Server) Listen() (err error) {
	ln, err := net.ListenTCP("tcp", s.addr)
	if err != nil {
		return
	}

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			s.logger.Printf("accept error %v\n", err)
			continue
		}

		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	host, err := s.request(conn)
	if err != nil {
		s.logger.Printf("request %v\n", err)
	}

	rconn, err := net.DialTimeout("tcp", host, s.timeout)
	if err != nil {
		s.logger.Printf("dial error %v\n", err)
		return
	}

	s.connect(rconn, conn)
}

func (s *Server) request(conn net.Conn) (host string, err error) {
	buf := s.pool.Get().([]byte)
	defer s.pool.Put(buf)

	// cmd 1, atyp 1, addr min 4, port 2
	// get min length 8
	_, err = io.ReadAtLeast(conn, buf, 8)
	if err != nil {
		return
	}

	var (
		dstAddr string
		dstPort int
	)
	switch buf[1] {
	case addrTypeIPV4:
		// addr range 2 - 4, length 8
		dstAddr = net.IP(buf[2:4]).String()
		dstPort = int(binary.BigEndian.Uint16(buf[4:8]))
	case addrTypeIPV6:
		// addr range 2 - 18, length 20
		dstAddr = net.IP(buf[2:18]).String()
		dstPort = int(binary.BigEndian.Uint16(buf[18:20]))
	case addrTypeDomain:
		// addr length 3
		dstAddrLen := buf[3]
		_, err = io.ReadFull(conn, buf[8:dstAddrLen+5])
		if err != nil {
			return
		}
		dstAddr = string(buf[4 : 4+dstAddrLen])
		dstPort = int(binary.BigEndian.Uint16(buf[3+dstAddrLen : 5+dstAddrLen]))
	default:
		err = errAddrType
		return
	}
	host = net.JoinHostPort(dstAddr, strconv.Itoa(dstPort))
	return
}

func (s *Server) connect(dst, src net.Conn) (err error) {
	errChan := make(chan error, 1)
	wg := sync.WaitGroup{}
	wg.Add(2)
	// read from src, write to remote
	go func() {
		defer func() {
			wg.Done()
		}()

		b := s.pool.Get().([]byte)
		for {
			select {
			case <-errChan:
				dst.Write([]byte{0})
				s.pool.Put(b)
				return
			default:
				n, err := src.Read(b)
				if n > 0 {
					dst.Write(b[:n])
				}

				if err != nil {
					errChan <- err
					continue
				}
			}
		}
	}()

	// read from remote, write to src
	go func() {
		defer func() {
			wg.Done()
		}()

		b := s.pool.Get().([]byte)
		for {
			select {
			case <-errChan:
				src.Write([]byte{0})
				s.pool.Put(b)
				return
			default:
				n, err := dst.Read(b)
				if n > 0 {
					src.Write(b[:n])
				}

				if err != nil {
					errChan <- err
					continue
				}
			}
		}
	}()

	wg.Wait()
	return
}
