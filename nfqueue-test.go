package main

import (
	"fmt"
	"github.com/chifflier/nfqueue-go/nfqueue"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

var strToSearch string
var strToReplace string

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s queue-number str-to-search str-to-replace\n", os.Args[0])
		os.Exit(1)
	}

	queueNumber, err := strconv.Atoi(os.Args[1])

	if err != nil {
		panic(err)
	}

	strToSearch, strToReplace = os.Args[2], os.Args[3]

	q := new(nfqueue.Queue)

	callback := func(payload *nfqueue.Payload) int {
		return handlePayload(payload, strToSearch, strToReplace)
	}

	q.SetCallback(callback)

	q.Init()
	defer q.Close()

	q.Unbind(syscall.AF_INET)
	q.Bind(syscall.AF_INET)

	q.CreateQueue(queueNumber)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			// sig is a ^C, handle it
			_ = sig
			q.Close()
			os.Exit(0)
		}
	}()

	q.TryRun()
}

func handlePayload(payload *nfqueue.Payload, searchStr, replaceStr string) int {
	packet := gopacket.NewPacket(payload.Data, layers.LayerTypeIPv4, gopacket.Default)

	appLayer := packet.ApplicationLayer()

	if len(packet.Layers()) != 3 || appLayer == nil {
		fmt.Printf("Packet SKIPPED %v\n", packet)
		payload.SetVerdict(nfqueue.NF_ACCEPT)
		return 0
	}

	ipLayer := packet.Layers()[0].(*layers.IPv4)
	tcpLayer := packet.Layers()[1].(*layers.TCP)

	fmt.Printf("Packet HANDLED %v\n", packet)

	buffer := gopacket.NewSerializeBuffer()
	tcpLayer.SetNetworkLayerForChecksum(ipLayer)
	options := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	newPayload := changePayload(appLayer, searchStr, replaceStr)

	err := gopacket.SerializeLayers(buffer, options,
		ipLayer,
		tcpLayer,
		newPayload,
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error serializing: %v\n", err)
	}

	payload.SetVerdictModified(nfqueue.NF_ACCEPT, buffer.Bytes())
	return 0
}

func changePayload(appLayer gopacket.ApplicationLayer, searchStr, replaceStr string) gopacket.Payload {
	text := string(appLayer.Payload())
	newText := strings.Replace(text, searchStr, replaceStr, -1)

	fmt.Println("OLD TEXT:\n", text)
	fmt.Println("NEW TEXT:\n", newText)

	return gopacket.Payload(newText)
}
