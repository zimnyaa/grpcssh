package main

import (
	"golang.org/x/crypto/ssh"
	"net"
	"io"
	"log"
	"context"
	"fmt"
	"os"
	"crypto/rand"
	"encoding/binary"
	"crypto/rsa"
	"syscall"
	"sync"
	"unsafe"
	"os/exec"
	"github.com/creack/pty"
	"zimnyaa/grpcssh/share"
	"google.golang.org/grpc"
	"zimnyaa/grpcssh/grpctun"
)


func main() {
	go sshexec()
	grpcconn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer grpcconn.Close()


	client := grpctun.NewTunnelServiceClient(grpcconn)
	stream, err := client.Tunnel(context.Background())
	if err != nil {
		log.Fatalf("Failed to open stream: %v", err)
	}
	nConn := share.NewGrpcClientConn(stream)


	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		panic("Failed to create signer")
	}
	config.AddHostKey(signer)

	sshConn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Fatalf("Failed to open stream: %v", err)
	}
	defer sshConn.Close()


	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		log.Printf("[socks] new channel\n")
		if newChannel.ChannelType() == "session" {
			go func() {
				connection, requests, err := newChannel.Accept()
				if err != nil {
					return
				}
				go ssh.DiscardRequests(requests)
				var domainBytes []byte = make([]byte, 1024)
				n, err := connection.Read(domainBytes)
				if err != nil || n == 0 {
					return
				}
				connection.Write(dnsResolve(string(domainBytes)))
				connection.Close()
			}()
			continue
		}

		if newChannel.ChannelType() != "direct-tcpip" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		

		var dReq struct {
			DestAddr string
			DestPort uint32
		}
		ssh.Unmarshal(newChannel.ExtraData(), &dReq)

		log.Printf("new direct-tcpip channel to %s:%d\n", dReq.DestAddr, dReq.DestPort)
		go func() {
			dest := fmt.Sprintf("%s:%d", dReq.DestAddr, dReq.DestPort)
			var conn net.Conn
			var err error
			if dReq.DestAddr == "1.1.1.1" {
				conn, err = net.Dial("unix", "/tmp/grpcssh")
			} else {
				conn, err = net.Dial("tcp", dest)
			}
				
			if err == nil {
				channel, chreqs, _ := newChannel.Accept()
				go ssh.DiscardRequests(chreqs)
	
				go func() {
					defer channel.Close()
					defer conn.Close()
					io.Copy(channel, conn)
				}()
				go func() {
					defer channel.Close()
					defer conn.Close()
					io.Copy(conn, channel)
				}()
			}
		}()
	}
	
}

func dnsResolve(name string) ([]byte) {
	fmt.Printf("dnsresolve: %s\n", name)
	addr, err := net.ResolveIPAddr("ip", name)
	if err != nil {
		return []byte("err")
	}
	return []byte(addr.IP.String())
}

// stolen from https://gist.github.com/jpillora/b480fde82bff51a06238
func sshexec() {
	os.Remove("/tmp/grpcssh")
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			return nil, nil
		},

	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		panic("Failed to create signer")
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("unix", "/tmp/grpcssh")
	if err != nil {
		log.Fatalf("Failed to listen (%s)", err)
	}

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection (%s)", err)
			continue
		}
		
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
		if err != nil {
			log.Printf("Failed to handshake (%s)", err)
			continue
		}

		log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		
		go ssh.DiscardRequests(reqs)
		
		go handleChannels(chans)
	}
}

func handleChannels(chans <-chan ssh.NewChannel) {
	
	for newChannel := range chans {
		go handleChannel(newChannel)
	}
}

func handleChannel(newChannel ssh.NewChannel) {

	if t := newChannel.ChannelType(); t != "session" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}

	
	bash := exec.Command("bash")

	
	close := func() {
		connection.Close()
		_, err := bash.Process.Wait()
		if err != nil {
			log.Printf("Failed to exit bash (%s)", err)
		}
		log.Printf("Session closed")
	}

	
	log.Print("Creating pty...")
	bashf, err := pty.Start(bash)
	if err != nil {
		log.Printf("Could not start pty (%s)", err)
		close()
		return
	}

	
	var once sync.Once
	go func() {
		io.Copy(connection, bashf)
		once.Do(close)
	}()
	go func() {
		io.Copy(bashf, connection)
		once.Do(close)
	}()

	
	go func() {
		for req := range requests {
			switch req.Type {
			case "shell":
				
				if len(req.Payload) == 0 {
					req.Reply(true, nil)
				}
			case "pty-req":
				termLen := req.Payload[3]
				w, h := parseDims(req.Payload[termLen+4:])
				SetWinsize(bashf.Fd(), w, h)
				
				req.Reply(true, nil)
			case "window-change":
				w, h := parseDims(req.Payload)
				SetWinsize(bashf.Fd(), w, h)
			}
		}
	}()
}

func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}

type Winsize struct {
	Height uint16
	Width  uint16
	x      uint16 // unused
	y      uint16 // unused
}

func SetWinsize(fd uintptr, w, h uint32) {
	ws := &Winsize{Width: uint16(w), Height: uint16(h)}
	syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}


