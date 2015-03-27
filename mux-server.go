package main

import (
	"fmt"
	"github.com/tarm/serial"
	"net"
	"os"
)

const (
	HOST = "localhost"
	PORT = "23200"
	TYPE = "tcp"
)

type Client struct {
	conn net.Conn
	ch   chan []byte
}

func mux(from_serial chan []byte, to_serial chan []byte,
	add_client chan Client, rm_client chan Client) {

	clients := make(map[net.Conn]chan []byte)

	for {
		select {
		case msg := <-from_serial:
			i := 0
			for _, ch := range clients {
				ch <- msg
				i += 1
			}

		case client := <-add_client:
			fmt.Println("MUX: New client", client.conn.RemoteAddr())
			clients[client.conn] = client.ch

		case client := <-rm_client:
			fmt.Println("MUX: Removing client", client.conn.RemoteAddr())
			delete(clients, client.conn)
		}
	}
}

func mux2client(c Client) {
	for msg := range c.ch {
		_, err := c.conn.Write(msg)
		if err != nil {
			return
		}
	}
}

func client2mux(c Client, to_serial chan []byte) {
	buf := make([]byte, 1024)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			return
		}
		to_serial <- buf[:n]
	}
}

func handle_client(conn net.Conn, to_serial chan []byte,
	add_client chan Client, rm_client chan Client) {

	client := Client{
		conn: conn,
		ch:   make(chan []byte),
	}

	defer func() {
		fmt.Println("Client: Connection closed", client.conn.RemoteAddr())
		client.conn.Close()
		rm_client <- client
	}()

	add_client <- client

	go mux2client(client)
	client2mux(client, to_serial)
}

func mux2serial(s *serial.Port, to_serial chan []byte) {
	for msg := range to_serial {
		_, err := s.Write(msg)
		if err != nil {
			fmt.Println("MUX> Error writing to serial port:", err.Error())
			os.Exit(1)
		}
	}
}

func serial2mux(s *serial.Port, from_serial chan []byte) {
	buf := make([]byte, 1024)
	for {
		n, err := s.Read(buf)
		if err != nil {
			fmt.Println("MUX> Error reading from serial port:", err.Error())
			os.Exit(1)
		}
		from_serial <- buf[:n]
	}
}

func handle_serial(from_serial chan []byte, to_serial chan []byte) {
	c := &serial.Config{Name: "/dev/ttyACM0", Baud: 9600}
	s, err := serial.OpenPort(c)
	if err != nil {
		fmt.Println("MUX> Error opening serial port:", err.Error())
		os.Exit(1)
	}
	defer s.Close()

	go mux2serial(s, to_serial)
	serial2mux(s, from_serial)
}

func main() {
	listener, err := net.Listen(TYPE, HOST+":"+PORT)
	if err != nil {
		fmt.Println("MUX> Error listening:", err.Error())
		os.Exit(1)
	}
	defer listener.Close()

	from_serial := make(chan []byte, 1024)
	to_serial := make(chan []byte, 1024)
	add_client := make(chan Client)
	rm_client := make(chan Client)

	go mux(from_serial, to_serial, add_client, rm_client)
	go handle_serial(from_serial, to_serial)

	fmt.Println("MUX> Listening on:", HOST+":"+PORT)
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("MUX> Error handling:", err.Error())
			continue
		}
		go handle_client(conn, to_serial, add_client, rm_client)
	}
}
