package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"hng-s1/src/utils"
	"net/http"
	"sync"
)

// Default Handler
func HandleNotFound(w http.ResponseWriter, r *http.Request) {
	utils.WriteError(w, http.StatusNotFound, "route not found")
}

func fetchAndDecode(ctx context.Context, client *http.Client, providerName, url string, target any, validate func() bool, wg *sync.WaitGroup, ch chan<- providerResponse) {
	defer wg.Done()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		ch <- providerResponse{name: providerName, err: err}
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		ch <- providerResponse{name: providerName, err: err}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ch <- providerResponse{name: providerName, err: fmt.Errorf("status %d", resp.StatusCode)}
		return
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		ch <- providerResponse{name: providerName, err: err}
		return
	}

	if !validate() {
		ch <- providerResponse{name: providerName, err: fmt.Errorf("%s returned an invalid response", providerName)}
		return
	}

	ch <- providerResponse{name: providerName}
}
