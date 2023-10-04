package main

import (
	"golang.org/x/crypto/ssh"
	"net"
	"fmt"
	"log"
	"context"
	"github.com/armon/go-socks5"
	"zimnyaa/grpcssh/share"
	"google.golang.org/grpc"
	"zimnyaa/grpcssh/grpctun"
)

type server struct{
	grpctun.UnimplementedTunnelServiceServer
}

func findUnusedPort(startPort int32) (int32) {
	for port := startPort; port <= 65535; port++ {
		addr := fmt.Sprintf("localhost:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		listener.Close()
		return port
	}
	return 0
}

func (s *server) Tunnel(stream grpctun.TunnelService_TunnelServer) error {
	log.Printf("new tunnel client\n")
	socksconn := share.NewGrpcServerConn(stream)

	sshConf := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("asdf")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	
	c, chans, reqs, err := ssh.NewClientConn(socksconn, "255.255.255.255", sshConf)
	if err != nil {
		fmt.Println("%v", err)
		return err
	}
	sshConn := ssh.NewClient(c, chans, reqs)
	
	defer sshConn.Close()

	log.Printf("connected to backwards ssh server\n")

	conf := &socks5.Config{
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return sshConn.Dial(network, addr)
		},
	}

	serverSocks, err := socks5.New(conf)
	if err != nil {
		fmt.Println(err)
		return err
	}
	port := findUnusedPort(1080)
	log.Printf("creating a socks server@%d\n", port)
	if err := serverSocks.ListenAndServe("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		log.Fatalf("failed to create socks5 server%v\n", err)
	}

	return nil

}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v\n", err)
	}
	s := grpc.NewServer()
	grpctun.RegisterTunnelServiceServer(s, &server{})
	s.Serve(lis)
}

