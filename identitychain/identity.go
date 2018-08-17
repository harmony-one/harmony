package identitychain

// IdentityChain (Blockchain) keeps Identities per epoch
type IdentityChain struct {
	Identities        []*IdentityBlock
	PendingIdentities []*Node
	Logger            log
}

// func main() {
// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	go func() {
// 		t := time.Now()
// 		genesisBlock := Block{}
// 		genesisBlock = Block{0, t.String(), 0, calculateHash(genesisBlock), "", difficulty, ""}

// 		mutex.Lock()
// 		Blockchain = append(Blockchain, genesisBlock)
// 		mutex.Unlock()
// 	}()
// 	log.Fatal(run())

// }

// // web server
// func run() error {
// 	mux := makeMuxRouter()
// 	httpPort := os.Getenv("PORT")
// 	log.Println("HTTP Server Listening on port :", httpPort)
// 	s := &http.Server{
// 		Addr:           ":" + httpPort,
// 		Handler:        mux,
// 		ReadTimeout:    10 * time.Second,
// 		WriteTimeout:   10 * time.Second,
// 		MaxHeaderBytes: 1 << 20,
// 	}

// 	if err := s.ListenAndServe(); err != nil {
// 		return err
// 	}

// 	return nil
// }
