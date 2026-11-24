// The pion/webrtc side of std/webrtc/interop_test_linux.low.
//
// Usage: pionpeer offer|answer. Descriptions travel as one hex line each on
// standard input and output. The peer echoes every data channel message, text
// with an "echo: " prefix, opens a channel "pion" that says "hello from pion",
// and sends every RTP packet it receives back on its own audio track. It exits
// when standard input closes.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/pion/ice/v4"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readDescription(in *bufio.Reader) string {
	line, err := in.ReadString('\n')
	must(err)
	raw, err := hex.DecodeString(strings.TrimSpace(line))
	must(err)
	return string(raw)
}

func writeDescription(pc *webrtc.PeerConnection) {
	fmt.Println(hex.EncodeToString([]byte(pc.LocalDescription().SDP)))
}

func setLocal(pc *webrtc.PeerConnection, d webrtc.SessionDescription) {
	gathered := webrtc.GatheringCompletePromise(pc)
	must(pc.SetLocalDescription(d))
	<-gathered
	writeDescription(pc)
}

func echo(dc *webrtc.DataChannel) {
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if msg.IsString {
			must(dc.SendText("echo: " + string(msg.Data)))
		} else {
			must(dc.Send(msg.Data))
		}
	})
}

func main() {
	var se webrtc.SettingEngine
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetIncludeLoopbackCandidate(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
	m := &webrtc.MediaEngine{}
	must(m.RegisterDefaultCodecs())
	api := webrtc.NewAPI(webrtc.WithSettingEngine(se), webrtc.WithMediaEngine(m))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	must(err)
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio", "pion")
	must(err)
	_, err = pc.AddTrack(track)
	must(err)
	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		for {
			p, _, err := remote.ReadRTP()
			if err != nil {
				return
			}
			must(track.WriteRTP(&rtp.Packet{Header: rtp.Header{Version: 2, PayloadType: p.PayloadType, SequenceNumber: p.SequenceNumber, Timestamp: p.Timestamp}, Payload: p.Payload}))
		}
	})
	pc.OnDataChannel(echo)
	hello, err := pc.CreateDataChannel("pion", nil)
	must(err)
	hello.OnOpen(func() { must(hello.SendText("hello from pion")) })
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) { fmt.Fprintln(os.Stderr, "state", s) })

	in := bufio.NewReader(os.Stdin)
	if os.Args[1] == "offer" {
		offer, err := pc.CreateOffer(nil)
		must(err)
		setLocal(pc, offer)
		must(pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: readDescription(in)}))
	} else {
		must(pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: readDescription(in)}))
		answer, err := pc.CreateAnswer(nil)
		must(err)
		setLocal(pc, answer)
	}
	in.ReadString('\n')
	must(pc.Close())
}
