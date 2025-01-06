package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"example.com/benchmark_proj/benchmark" // Import the generated Protobuf package

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type Data struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// REST API Handler
func restHandler(w http.ResponseWriter, r *http.Request) {
	var data Data
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	data.Value += 1 // Simulate processing
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// gRPC Service Implementation
type rpcService struct {
	benchmark.UnimplementedAPIServer
}

func (s *rpcService) SendData(ctx context.Context, req *benchmark.Data) (*benchmark.Data, error) {
	req.Value += 1 // Simulate processing
	return req, nil
}

// Benchmark REST API
func benchmarkRESTAPI(url string, data *Data, iterations int) {
	jsonData, _ := json.Marshal(data)

	start := time.Now()
	for i := 0; i < iterations; i++ {
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Fatalf("REST API call failed: %v", err)
		}
		resp.Body.Close()
	}
	duration := time.Since(start)
	fmt.Printf("REST API Total Time for %d iterations: %v\n", iterations, duration)
	fmt.Printf("REST API Average Time per Call: %v\n", duration/time.Duration(iterations))
}

// Benchmark gRPC API
func benchmarkRPCAPI(client benchmark.APIClient, data *benchmark.Data, iterations int) {
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, err := client.SendData(context.Background(), data)
		if err != nil {
			log.Fatalf("gRPC API call failed: %v", err)
		}
	}
	duration := time.Since(start)
	fmt.Printf("RPC API Total Time for %d iterations: %v\n", iterations, duration)
	fmt.Printf("RPC API Average Time per Call: %v\n", duration/time.Duration(iterations))
}

// Measure PayloadSize
func measurePayloadSize(data *benchmark.Data) {
    // JSON Serialization
    jsonData, _ := json.Marshal(data)
    fmt.Printf("JSON Payload Size: %d bytes\n", len(jsonData))

    // Protobuf Serialization
    protoData, _ := proto.Marshal(data)
    fmt.Printf("Protobuf Payload Size: %d bytes\n", len(protoData))
}

// Concurrency function to benchmark both APIs
func benchmarkConcurrencyRPC(client benchmark.APIClient, data *benchmark.Data, concurrency int, iterations int) {
	var wg sync.WaitGroup
	ch := make(chan time.Duration, iterations)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations/concurrency; j++ {
				start := time.Now()
				_, err := client.SendData(context.Background(), data)
				if err != nil {
					log.Printf("gRPC API call failed: %v", err)
					continue
				}
				ch <- time.Since(start)
			}
		}()
	}

	wg.Wait()
	close(ch)

	// Calculate total and average latency
	var totalDuration time.Duration
	for latency := range ch {
		totalDuration += latency
	}
	fmt.Printf("gRPC API - Total Time: %v\n", totalDuration)
	fmt.Printf("gRPC API - Average Time per Call: %v\n", totalDuration/time.Duration(iterations))
}

func benchmarkConcurrencyREST(url string, data *Data, concurrency int, iterations int) {
	var wg sync.WaitGroup
	ch := make(chan time.Duration, iterations)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations/concurrency; j++ {
				start := time.Now()
				jsonData, _ := json.Marshal(data)
				resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					log.Printf("REST API call failed: %v", err)
					continue
				}
				resp.Body.Close()
				ch <- time.Since(start)
			}
		}()
	}

	wg.Wait()
	close(ch)

	// Calculate total and average latency
	var totalDuration time.Duration
	for latency := range ch {
		totalDuration += latency
	}
	fmt.Printf("REST API - Total Time: %v\n", totalDuration)
	fmt.Printf("REST API - Average Time per Call: %v\n", totalDuration/time.Duration(iterations))
}


func main() {
	// Set up REST API
	go func() {
		http.HandleFunc("/api", restHandler)
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Set up gRPC API
	grpcServer := grpc.NewServer()
	benchmark.RegisterAPIServer(grpcServer, &rpcService{})

	go func() {
		listener, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		log.Fatal(grpcServer.Serve(listener))
	}()

	// Allow servers to start
	time.Sleep(2 * time.Second)

	// Benchmark Setup
	data := &Data{ID: "123", Name: "Test", Value: 42}
	protoData := &benchmark.Data{Id: "123", Name: "Test", Value: 42}
	iterations := 100

	// Measure Payload Size
	fmt.Println("Measure Payload size...")
	measurePayloadSize(protoData)

	// Benchmark REST API
	fmt.Println("Benchmarking REST API...")
	benchmarkRESTAPI("http://localhost:8080/api", data, iterations)

	// Benchmark gRPC API
	fmt.Println("Benchmarking RPC API...")
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()
	client := benchmark.NewAPIClient(conn)
	benchmarkRPCAPI(client, protoData, iterations)


	// Benchmark Parameters
	concurrency := 10

	// Run Concurrency Benchmarks
	fmt.Println("Benchmarking gRPC API with Concurrency...")
	benchmarkConcurrencyRPC(client, protoData, concurrency, iterations)

	fmt.Println("Benchmarking REST API with Concurrency...")
	benchmarkConcurrencyREST("http://localhost:8080/api", data, concurrency, iterations)

}

