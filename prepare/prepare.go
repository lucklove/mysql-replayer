package prepare

import (
	"os"
	"fmt"
	"flag"
	"context"
	"math/rand"
	"github.com/google/subcommands"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/lucklove/mysql-replayer/utils"
)

type Record struct {
	file *os.File
	fileEmpty bool
	lastTcpSeq uint32
	lastTcpLen uint32
	unresolvedData []byte
	expectLen int
	dropping bool
}

type PrepareCommand struct {
	input string
	output string
	records map[string]*Record
}
  
func (*PrepareCommand) Name() string     { return "prepare" }
func (*PrepareCommand) Synopsis() string { return "Translate .pcap file to input file of next stage." }
func (*PrepareCommand) Usage() string {
	return `prepare -i pcap-file -o output-dir:
	Analyze pcap file to connection streams.
	`
}
  
func (p *PrepareCommand) SetFlags(f *flag.FlagSet) {
	f.StringVar(&p.input, "i", "", "input pcap file")
	f.StringVar(&p.output, "o", "", "output directory")
}
  
func (p *PrepareCommand) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(p.output) == 0 || len(p.input) == 0 {
		fmt.Println(p.Usage())
		return subcommands.ExitSuccess
	}

	if err := utils.EnsureDir(p.output); err != nil {
		fmt.Println(err)
		return subcommands.ExitFailure
	}

	p.records = make(map[string]*Record)

	if handle, err := pcap.OpenOffline(p.input); err != nil {
		panic(err)
	} else {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for packet := range packetSource.Packets() {
			p.handlePacket(packet)
		}
	}

	return subcommands.ExitSuccess
}

func (p *PrepareCommand) createRecord(metadata *gopacket.PacketMetadata, ip *layers.IPv4, tcp *layers.TCP) {
	fname := fmt.Sprintf("%d-%s-%s-%d.rec", 
						metadata.Timestamp.Unix(), 
						ip.SrcIP, 
						tcp.SrcPort,
						rand.Intn(1000000))
	fpath := fmt.Sprintf("%s/%s", p.output, fname)

	if f, err := os.Create(fpath); err == nil {
		identity := fmt.Sprintf("%s-%d", ip.SrcIP, tcp.SrcPort)

		// TCP SYN resend or FIN lost
		if r, ok := p.records[identity]; ok {
			p.deleteRecord(r, ip, tcp)
		}

		p.records[identity] = &Record {
			file: f,
			fileEmpty: true,
			lastTcpSeq: tcp.Seq,
			lastTcpLen: uint32(len(tcp.Payload)),
			unresolvedData: []byte{},
			expectLen: 0,
			dropping: false,
		}
	}
}

func (p *PrepareCommand) deleteRecord(r *Record, ip *layers.IPv4, tcp *layers.TCP) {
	identity := fmt.Sprintf("%s-%d", ip.SrcIP, tcp.SrcPort)

	r.file.Close()
	delete(p.records, identity)
}


func (p *PrepareCommand) handlePackageLost(r *Record, tcp *layers.TCP, metadata *gopacket.PacketMetadata) {
	if r.expectLen != 0 {
		r.expectLen = 0
	}

	utils.InsertSQLToFile(r.file, `select "package lost";`, metadata.Timestamp.Unix())

	r.dropping = true

	if tcp.Payload[4] == 3 {
		p.handleQuery(r, tcp, metadata)
	}
}

func (p *PrepareCommand) handleAuthentication(r *Record, tcp *layers.TCP) {
	r.file.WriteString(fmt.Sprintf("%s\n", utils.ReadConnectDBName(tcp.Payload[4:])))
	r.fileEmpty = false
}

func (p *PrepareCommand) appendUnresolvedData(r *Record, tcp *layers.TCP, metadata *gopacket.PacketMetadata) {
	r.unresolvedData = append(r.unresolvedData, tcp.Payload...)
	if r.expectLen <= int(len(r.unresolvedData)) {
		utils.InsertSQLToFile(r.file, string(r.unresolvedData), metadata.Timestamp.Unix())
		r.expectLen = 0
	}
}

func (p *PrepareCommand) handleQuery(r *Record, tcp *layers.TCP, metadata *gopacket.PacketMetadata) {
	data := tcp.Payload
	l := int(uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16)

	// Try to recover from package lost by droping any suspected truncated frame
	if r.dropping && l != len(data[4:]) {
		return
	}

	if l > len(data[4:]) {
		r.expectLen = l - 1
		r.unresolvedData = data[5:]
	} else {
		utils.InsertSQLToFile(r.file, string(data[5:]), metadata.Timestamp.Unix())
	}

	if r.dropping {
		r.dropping = false
	}
}

func (p *PrepareCommand) handlePacket(packet gopacket.Packet) {
	metadata, ip, tcp := utils.ReadLayers(packet)
	if metadata == nil || ip == nil || tcp == nil {
		return
	}

	if tcp.SYN {
		p.createRecord(metadata, ip, tcp)
		return
	}

	identity := fmt.Sprintf("%s-%d", ip.SrcIP, tcp.SrcPort)
	if r, ok := p.records[identity]; ok {
		// Connection closed
		if tcp.FIN  {
			p.deleteRecord(r, ip, tcp)
			return
		}

		// Tcp reorder
		if tcp.Seq < r.lastTcpSeq {
			return
		}

		tcpLen := uint32(len(tcp.Payload))
		r.lastTcpSeq = tcp.Seq
		r.lastTcpLen = tcpLen
		
		if tcpLen == 0 {
			return
		}

		// Package lost
		if tcp.Seq > r.lastTcpSeq + r.lastTcpLen {
			if r.fileEmpty {
				// If the authentication request lost, the connection will be droped
				p.deleteRecord(r, ip, tcp)
			} else {
				// Else try to recover from package missing
				p.handlePackageLost(r, tcp, metadata)
			}
			return
		} 

		if r.fileEmpty {
			p.handleAuthentication(r, tcp)
		} else if r.expectLen != 0 {		// There is data unresolved
			p.appendUnresolvedData(r, tcp, metadata)
		} else if tcp.Payload[4] == 3 { 	// Only handle query
			p.handleQuery(r, tcp, metadata)
		}
	}
}