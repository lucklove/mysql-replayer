package utils

import (
	"bytes"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const CLIENT_CONNECT_WITH_DB = 0x00000008
const CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA = 0x00200000
const CLIENT_SECURE_CONNECTION = 0x00008000

func ReadLayers(packet gopacket.Packet) (metadata *gopacket.PacketMetadata, ip *layers.IPv4, tcp *layers.TCP) {
	metadata = packet.Metadata()

	// TODO: handle IPV6
	if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		ip, _ = ipv4Layer.(*layers.IPv4)
	}

	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ = tcpLayer.(*layers.TCP)
	}

	return
}

// Read dbname from authentication request
func ReadConnectDBName(data []byte) (dbname string) {
	capabilities := int(uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24)

	if (capabilities & CLIENT_CONNECT_WITH_DB) == 0 {
		return ""
	}

	pos := 32
	pos += bytes.IndexByte(data[pos:], 0x00) + 1

	if (capabilities&CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA) == 0 &&
		(capabilities&CLIENT_SECURE_CONNECTION) == 0 {
		pos += bytes.IndexByte(data[pos:], 0x00) + 1
	} else {
		pos += int(data[pos]) + 1
	}

	l := bytes.IndexByte(data[pos:], 0x00)
	return string(data[pos : pos+l])
}
