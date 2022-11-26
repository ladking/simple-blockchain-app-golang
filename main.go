package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

//blockchain app of a healthcare company collecting pulse data of heartbeat
// struct of each block that will make up the blockchain

type Block struct {
	Index     int    //position of data record in the blockchain
	Timestamp string //time data is written, (automatically determined)
	BPM       int    // beat per minute is your pulse rate
	Hash      string //sha256 identifier representing the data record
	PrevHash  string //sha256 identifier of the previous record in the chain
}

// blockchain itself is a slice of data
var Blockchain []Block

//How does hashing fit into blocks and blockchanin?? we use hashes to identify and keep the blocks in the right order
//By ensuring the prevhash in each block is identical to the hash in preceding block

//WHY DO WE NEED HASHING
//1-- To save space-- hashes are derived from all the data that is on the block, its more efficient to hash data
//into single sha256 string or hash the hashes than to copy all the data in preceding blocks over and over again

//2-- In other to preserve the integrity of the blockchain. By storing the previous hashes in the blockchain
// we are able to ensure the blocks are in the right order.
//If a malicious party were to come in and try to manipulate the data
//the hashes would change quickly and the chain would “break”, and everyone would know to not trust that malicious chain.

// function that takes our block data and create a sha256 hash

func calculateHash(block Block) string {
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash

	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)

	return hex.EncodeToString(hashed)
}

func generateBlock(oldBlock Block, BPM int) (Block, error) {
	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = calculateHash(newBlock)

	return newBlock, nil
}

//Note in the generatBlock function time is automatically generated
//Index is also incremented from the index of previous block.

//Block Validation
//function to ensure that block has not been tampered with

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateHash(newBlock) != newBlock.Hash {
		return false
	}
	return true
}

// issue when two nodes of our blockchain ecosystem both added block to their chain and we recieve them both
// we choose the longest chain as the source of truth
func replaceChain(newBlocks []Block) {
	if len(newBlocks) > len(Blockchain) {
		Blockchain = newBlocks
	}
}

func run() error {
	mux := makeMuxRouter()

	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	httpAddr := os.Getenv("PORT")
	log.Println("Listening on", os.Getenv("PORT"))

	s := &http.Server{
		Addr:           httpAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	if err := s.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

func makeMuxRouter() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/", readFromBlockchain).Methods("GET")
	router.HandleFunc("/", writeToBlockchain).Methods("POST")
	return router
}

func readFromBlockchain(res http.ResponseWriter, req *http.Request) {
	bytes, err := json.Marshal(Blockchain)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(res, string(bytes))
}

type Message struct {
	BPM int
}

func writeToBlockchain(res http.ResponseWriter, req *http.Request) {
	var m Message

	err := json.NewDecoder(req.Body).Decode(&m)
	if err != nil {
		respondWithJson(res, req, http.StatusBadRequest, req.Body)
		return
	}
	defer req.Body.Close()

	newBlock, err := generateBlock(Blockchain[len(Blockchain)-1], m.BPM)
	if err != nil {
		respondWithJson(res, req, http.StatusBadRequest, req.Body)
		return
	}
	if isBlockValid(newBlock, Blockchain[len(Blockchain)-1]) {
		newBlockchain := append(Blockchain, newBlock)
		replaceChain(newBlockchain)
		spew.Dump(Blockchain)
	}

	respondWithJson(res, req, http.StatusCreated, newBlock)
}

func respondWithJson(w http.ResponseWriter, r *http.Request, code int, payload interface{}) {
	response, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("HTTP 500: Internal Server Error"))
		return
	}
	w.WriteHeader(code)
	w.Write(response)
}

func main() {
	go func() {
		t := time.Now()
		firstblock := Block{0, t.String(), 0, "", ""}
		spew.Dump(firstblock)
		Blockchain = append(Blockchain, firstblock)
	}()

	log.Fatal(run())
}
