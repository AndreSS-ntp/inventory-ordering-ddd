// Command loadtest fires N concurrent POST /orders requests at a running
// Ordering instance, all targeting the same SKU, and reports:
//   - number of successful (reserved) orders
//   - number of failed orders (with a breakdown by reason)
//   - average number of Inventory read-check-CAS attempts per successful order
//   - total wall-clock time
//
// Output is printed as a human-readable table and as CSV (stdout), so it can
// be pasted directly into a report.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

type orderRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

type orderResponse struct {
	ID                string `json:"id"`
	SKU               string `json:"sku"`
	Quantity          int    `json:"quantity"`
	Status            string `json:"status"`
	FailureReason     string `json:"failure_reason,omitempty"`
	CreatedAt         string `json:"created_at"`
	InventoryAttempts int    `json:"inventory_attempts"`
}

type result struct {
	status   string
	reason   string
	attempts int
	err      error
	latency  time.Duration
}

func main() {
	url := flag.String("url", "http://localhost:8080/orders", "Ordering HTTP endpoint")
	sku := flag.String("sku", "SKU-002", "SKU to hammer concurrently")
	quantity := flag.Int("quantity", 1, "quantity requested per order")
	clients := flag.Int("clients", 50, "number of concurrent clients")
	timeout := flag.Duration("timeout", 10*time.Second, "per-request HTTP timeout")
	flag.Parse()

	httpClient := &http.Client{Timeout: *timeout}

	results := make([]result, *clients)
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = placeOrder(httpClient, *url, *sku, *quantity)
		}(i)
	}
	wg.Wait()
	totalDuration := time.Since(start)

	report(results, totalDuration, *clients, *sku)
}

func placeOrder(client *http.Client, url, sku string, quantity int) result {
	reqStart := time.Now()
	body, _ := json.Marshal(orderRequest{SKU: sku, Quantity: quantity})

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return result{status: "error", err: err, latency: time.Since(reqStart)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return result{status: "error", err: fmt.Errorf("unexpected HTTP status %d", resp.StatusCode), latency: time.Since(reqStart)}
	}

	var parsed orderResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return result{status: "error", err: err, latency: time.Since(reqStart)}
	}

	return result{
		status:   parsed.Status,
		reason:   parsed.FailureReason,
		attempts: parsed.InventoryAttempts,
		latency:  time.Since(reqStart),
	}
}

func report(results []result, totalDuration time.Duration, clients int, sku string) {
	var (
		reservedCount  int
		failedCount    int
		errorCount     int
		totalAttempts  int
		failureReasons = map[string]int{}
	)

	for _, r := range results {
		switch {
		case r.err != nil:
			errorCount++
		case r.status == "reserved":
			reservedCount++
			totalAttempts += r.attempts
		case r.status == "failed":
			failedCount++
			failureReasons[r.reason]++
		}
	}

	avgAttempts := 0.0
	if reservedCount > 0 {
		avgAttempts = float64(totalAttempts) / float64(reservedCount)
	}

	fmt.Println("=== Load test report ===")
	fmt.Printf("SKU:                        %s\n", sku)
	fmt.Printf("Concurrent clients:         %d\n", clients)
	fmt.Printf("Successful (reserved):      %d\n", reservedCount)
	fmt.Printf("Failed (business reject):   %d\n", failedCount)
	fmt.Printf("Errors (transport/HTTP):    %d\n", errorCount)
	fmt.Printf("Avg Inventory attempts/success: %.2f\n", avgAttempts)
	fmt.Printf("Total wall-clock time:      %s\n", totalDuration)
	if len(failureReasons) > 0 {
		fmt.Println("Failure reasons:")
		reasons := make([]string, 0, len(failureReasons))
		for reason := range failureReasons {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			fmt.Printf("  %-30s %d\n", reason, failureReasons[reason])
		}
	}

	fmt.Println()
	fmt.Println("=== CSV ===")
	fmt.Println("sku,clients,reserved,failed,errors,avg_attempts_per_success,total_duration_ms")
	fmt.Printf("%s,%d,%d,%d,%d,%.2f,%d\n",
		sku, clients, reservedCount, failedCount, errorCount, avgAttempts, totalDuration.Milliseconds())

	if errorCount > 0 {
		os.Exit(1)
	}
}
