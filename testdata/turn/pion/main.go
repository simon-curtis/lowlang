// The TURN server of std/ice/session/relay_test_linux.low, built on pion/turn.
//
// It listens on 127.0.0.1 at a free UDP port, a free TCP port and a free TLS
// port with the certificate and key in the PEM files named by its two
// arguments, prints the three ports on one line, relays on 127.0.0.1 for the
// user "alice" with the password "secret" in the realm "low", and exits when
// standard input closes.
package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"os"

	"github.com/pion/turn/v5"
)

func main() {
	certificate, err := tls.LoadX509KeyPair(os.Args[1], os.Args[2])
	if err != nil {
		panic(err)
	}
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	plain, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	secure, err := tls.Listen("tcp4", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}})
	if err != nil {
		panic(err)
	}
	relay := func() turn.RelayAddressGenerator {
		return &turn.RelayAddressGeneratorStatic{RelayAddress: net.ParseIP("127.0.0.1"), Address: "127.0.0.1"}
	}
	key := turn.GenerateAuthKey("alice", "low", "secret")
	server, err := turn.NewServer(turn.ServerConfig{
		Realm: "low",
		AuthHandler: func(ra *turn.RequestAttributes) (string, []byte, bool) {
			return ra.Username, key, ra.Username == "alice"
		},
		PacketConnConfigs: []turn.PacketConnConfig{{PacketConn: conn, RelayAddressGenerator: relay()}},
		ListenerConfigs: []turn.ListenerConfig{
			{Listener: plain, RelayAddressGenerator: relay()},
			{Listener: secure, RelayAddressGenerator: relay()},
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(conn.LocalAddr().(*net.UDPAddr).Port, plain.Addr().(*net.TCPAddr).Port, secure.Addr().(*net.TCPAddr).Port)
	bufio.NewReader(os.Stdin).ReadString('\n')
	server.Close()
}
