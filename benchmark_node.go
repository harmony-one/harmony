package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"harmony-benchmark/consensus"
	"harmony-benchmark/p2p"
)

// Consts
const (
	Message       = "Ping"
	StopCharacter = "\r\n\r\n"
)

// Start a server and process the request by a handler.
func startServer(port string, handler func(net.Conn, *consensus.Consensus), consensus *consensus.Consensus) {
	listen, err := net.Listen("tcp4", ":"+port)
	defer listen.Close()
	if err != nil {
		log.Fatalf("Socket listen port %s failed,%s", port, err)
		os.Exit(1)
	}
	log.Printf("Begin listen port: %s", port)
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Printf("Error listening on port: %s. Exiting.", port)
			log.Fatalln(err)
			continue
		}
		go handler(conn, consensus)
	}
}

func relayToPorts(msg string, conn net.Conn) {
	w := bufio.NewWriter(conn)
	ports := convertIntoInts(msg)
	ch := make(chan int)
	for i, port := range ports {
		go Send(port, strconv.Itoa(i), ch)
	}
	count := 0
	for count < len(ports) {
		log.Println(<-ch)
		count++
	}
	w.Write([]byte(Message))
	w.Flush()
	log.Printf("Send: %s", Message)

}

// Helper library to convert '1,2,3,4' into []int{1,2,3,4}.
func convertIntoInts(data string) []int {
	var res = []int{}
	items := strings.Split(data, " ")
	for _, value := range items {
		intValue, err := strconv.Atoi(value)
		checkError(err)
		res = append(res, intValue)
	}
	return res
}

// Do check error.
func checkError(err error) {
	if err != nil {
		log.Fatalln(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

// SocketClient is to connect a socket given a port and send the given message.
func SocketClient(ip, message string, port int) (res string) {
	addr := strings.Join([]string{ip, strconv.Itoa(port)}, ":")
	conn, err := net.Dial("tcp", addr)

	defer conn.Close()

	if err != nil {
		log.Fatalln(err)
	}

	conn.Write([]byte(message))
	conn.Write([]byte(StopCharacter))
	log.Printf("Send: %s", Message)

	buff := make([]byte, 1024)
	n, _ := conn.Read(buff)
	res = string(buff[:n])
	return
}

//https://gist.github.com/kenshinx/5796276
// Send a message to another node with given port.
func Send(port int, message string, ch chan int) (returnMessage string) {
	ip := "127.0.0.1"
	returnMessage = SocketClient(ip, message, port)
	ch <- port
	log.Println(returnMessage)
	return
}

// Handler of the leader node.
func NodeHandler(conn net.Conn, consensus *consensus.Consensus) {
	defer conn.Close()

	payload, err := p2p.ReadMessagePayload(conn)
	if err != nil {
		if consensus.IsLeader {
			log.Fatalf("[Leader] Receive data failed:%s", err)
		} else {
			log.Fatalf("[Slave] Receive data failed:%s", err)
		}
	}
	if consensus.IsLeader {
		consensus.ProcessMessageLeader(payload)
	} else {
		consensus.ProcessMessageValidator(payload)
	}
	//relayToPorts(receivedMessage, conn)
}

func getLeader(iplist string) p2p.Peer {
	file, _ := os.Open(iplist)
	fscanner := bufio.NewScanner(file)
	var leaderPeer p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" {
			leaderPeer.Ip = ip
			leaderPeer.Port = port
		}
	}
	return leaderPeer
}

func getPeers(Ip, Port, iplist string) []p2p.Peer {
	file, _ := os.Open(iplist)
	fscanner := bufio.NewScanner(file)
	var peerList []p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" || ip == Ip && port == Port {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP of the node")
	port := flag.String("port", "9000", "port of the node.")
	ipfile := flag.String("ipfile", "iplist.txt", "file containing all ip addresses")
	flag.Parse()
	log.Println()

	consensusObj := consensus.InitConsensus(*ip, *port, getPeers(*ip, *port, *ipfile), getLeader(*ipfile))
	var nodeStatus string
	if consensusObj.IsLeader {
		nodeStatus = "leader"
	} else {
		nodeStatus = "validator"
	}
	log.Println(consensusObj)
	log.Printf("This node is a %s node with ip: %s and port: %s\n", nodeStatus, *ip, *port)
	log.Println()
	startServer(*port, NodeHandler, &consensusObj)
}
