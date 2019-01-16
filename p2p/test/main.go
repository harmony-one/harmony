package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host/hostv2"
)

var (
	host *hostv2.HostV2
)

func main() {
	port := flag.String("p", "3000", "port of the node.")
	boot := flag.String("b", "", "addr of bootnode")
	flag.Parse()

	peer := p2p.Peer{IP: "0.0.0.0", Port: *port}
	host = hostv2.New(peer)
	if *boot != "" {
		log.Printf("bootnode: %s", *boot)
		host.AddBootNode(*boot)
	}
	host.SetRendezvousString("tt")
	host.BindHandler(handleStream)
	go ping()
	host.BindHandlerAndServe(handleStream)

}

func handleStream(s p2p.Stream) {
	// msg, err := bufio.NewReader(s).ReadString('\n')
	// if err != nil {
	// 	panic(err)
	// }
	// log.Printf("Received msg: %s", msg)

	log.Println("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

	go readData(rw)
	go writeData(rw)
}

func ping() {
	// for range time.Tick(3 * time.Second) {
	// 	host.Broadcast()
	// }
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from buffer")
			panic(err)
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			fmt.Println("Error writing to buffer")
			panic(err)
		}
		err = rw.Flush()
		if err != nil {
			fmt.Println("Error flushing buffer")
			panic(err)
		}
	}
}
