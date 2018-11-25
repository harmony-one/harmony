package main

import (
	"context"
	"flag"
	"log"
	"time"

	pb "github.com/harmony-one/harmony/syncing/downloader/proto"
	"google.golang.org/grpc"
)

// printResult ...
func printResult(client pb.DownloaderClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := &pb.DownloaderRequest{Type: pb.DownloaderRequest_HEADER}
	response, err := client.Query(ctx, request)
	if err != nil {
		log.Fatalf("Error")
	}
	log.Println(response)
}

func main() {
	flag.Parse()
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	conn, err := grpc.Dial("localhost:9999", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	defer conn.Close()
	client := pb.NewDownloaderClient(conn)

	printResult(client)
}
