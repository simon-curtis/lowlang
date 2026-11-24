import os
import struct
import sys

import gi

gi.require_version("Gst", "1.0")
from gi.repository import Gst

Gst.init(None)

SSRC = 0x12345678
PAYLOAD_TYPE = 96


def sender_report():
    block = struct.pack(">IIIIII", 0x0A0B0C0D, 0x19FFFFFE, 0x00010005, 7, 0x11223344, 0x00010000)
    info = struct.pack(">IIIIII", SSRC, 0xE5000000, 0x80000000, 0x1000, 10, 1200)
    body = info + block
    return struct.pack(">BBH", 0x81, 200, len(body) // 4) + body


def key_property(key):
    return Gst.Buffer.new_wrapped(key)


def drain(sink, directory, pattern, limit_ns):
    count = 0
    while True:
        sample = sink.emit("try-pull-sample", limit_ns)
        if sample is None:
            return count
        buf = sample.get_buffer()
        ok, info = buf.map(Gst.MapFlags.READ)
        data = bytes(info.data)
        buf.unmap(info)
        with open(os.path.join(directory, pattern.format(count)), "wb") as f:
            f.write(data)
        count += 1


def new_sink(name):
    sink = Gst.ElementFactory.make("appsink", name)
    sink.set_property("sync", False)
    sink.set_property("emit-signals", False)
    return sink


def encrypt(cipher, rtp_auth, rtcp_auth, key, directory, first_sequence, count):
    enc = Gst.ElementFactory.make("srtpenc", "enc")
    enc.set_property("key", key_property(key))
    enc.set_property("rtp-cipher", cipher)
    enc.set_property("rtcp-cipher", cipher)
    enc.set_property("rtp-auth", rtp_auth)
    enc.set_property("rtcp-auth", rtcp_auth)
    pipeline = Gst.Pipeline.new("encrypt")
    source = Gst.parse_bin_from_description(
        "audiotestsrc wave=silence num-buffers=%d samplesperbuffer=8 ! audio/x-raw,rate=8000,channels=1,format=S16BE"
        " ! rtpL16pay pt=%d seqnum-offset=%d timestamp-offset=0 ssrc=%d" % (count, PAYLOAD_TYPE, first_sequence, SSRC),
        True,
    )
    control = Gst.ElementFactory.make("appsrc", "control")
    control.set_property("caps", Gst.Caps.from_string("application/x-rtcp"))
    media_out = new_sink("media")
    control_out = new_sink("control_out")
    for element in (source, control, enc, media_out, control_out):
        pipeline.add(element)
    source.link_pads("src", enc, "rtp_sink_0")
    control.link_pads("src", enc, "rtcp_sink_0")
    enc.link_pads("rtp_src_0", media_out, "sink")
    enc.link_pads("rtcp_src_0", control_out, "sink")
    pipeline.set_state(Gst.State.PLAYING)
    control.emit("push-buffer", Gst.Buffer.new_wrapped(sender_report()))
    control.emit("end-of-stream")
    sent = drain(media_out, directory, "{:03d}.srtp", 3 * Gst.SECOND)
    drain(control_out, directory, "rtcp{}.srtcp", Gst.SECOND)
    pipeline.set_state(Gst.State.NULL)
    print("protected", sent)


def decrypt_stream(caps, pad, inputs, directory, outputs):
    pipeline = Gst.Pipeline.new("decrypt")
    source = Gst.ElementFactory.make("appsrc", "source")
    source.set_property("caps", Gst.Caps.from_string(caps))
    dec = Gst.ElementFactory.make("srtpdec", "dec")
    sink = new_sink("sink")
    for element in (source, dec, sink):
        pipeline.add(element)
    source.link_pads("src", dec, pad + "_sink")
    dec.link_pads(pad + "_src", sink, "sink")
    pipeline.set_state(Gst.State.PLAYING)
    index = 0
    while True:
        path = os.path.join(directory, inputs.format(index))
        if not os.path.exists(path):
            break
        with open(path, "rb") as f:
            source.emit("push-buffer", Gst.Buffer.new_wrapped(f.read()))
        index += 1
    source.emit("end-of-stream")
    opened = drain(sink, directory, outputs, 3 * Gst.SECOND)
    pipeline.set_state(Gst.State.NULL)
    return opened


def decrypt(cipher, rtp_auth, rtcp_auth, key, directory):
    fields = (
        "srtp-key=(buffer)%s,srtp-cipher=(string)%s,srtp-auth=(string)%s,"
        "srtcp-cipher=(string)%s,srtcp-auth=(string)%s"
    ) % (key.hex(), cipher, rtp_auth, cipher, rtcp_auth)
    rtp_caps = "application/x-srtp,payload=(int)%d,ssrc=(uint)%d,clock-rate=(int)8000,%s" % (PAYLOAD_TYPE, SSRC, fields)
    rtcp_caps = "application/x-srtcp,ssrc=(uint)%d,%s" % (SSRC, fields)
    opened = decrypt_stream(rtp_caps, "rtp", "{:03d}.srtp", directory, "{:03d}.rtp")
    decrypt_stream(rtcp_caps, "rtcp", "rtcp{}.srtcp", directory, "rtcp{}.rtcp")
    print("unprotected", opened)


def main():
    mode, cipher, rtp_auth, rtcp_auth, key_hex, directory = sys.argv[1:7]
    key = bytes.fromhex(key_hex)
    if mode == "encrypt":
        encrypt(cipher, rtp_auth, rtcp_auth, key, directory, int(sys.argv[7]), int(sys.argv[8]))
    else:
        decrypt(cipher, rtp_auth, rtcp_auth, key, directory)
    sys.stdout.flush()
    os._exit(0)


main()
