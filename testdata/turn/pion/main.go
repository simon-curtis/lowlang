// The TURN server of std/ice/session/relay_test_linux.low, built on pion/turn.
//
// It listens on 127.0.0.1 at a free UDP port, prints the port on standard
// output, relays on 127.0.0.1 for the user "alice" with the password "secret"
// in the realm "low", and exits when standard input closes.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"

	"github.com/pion/turn/v5"
)

func main() {
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	key := turn.GenerateAuthKey("alice", "low", "secret")
	server, err := turn.NewServer(turn.ServerConfig{
		Realm: "low",
		AuthHandler: func(ra *turn.RequestAttributes) (string, []byte, bool) {
			return ra.Username, key, ra.Username == "alice"
		},
		PacketConnConfigs: []turn.PacketConnConfig{{
			PacketConn:            conn,
			RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{RelayAddress: net.ParseIP("127.0.0.1"), Address: "127.0.0.1"},
		}},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(conn.LocalAddr().(*net.UDPAddr).Port)
	bufio.NewReader(os.Stdin).ReadString('\n')
	server.Close()
}
