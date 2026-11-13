package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUnknownPathReturnsNotFound(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(NewRouter())
	t.Cleanup(server.Close)

	client := server.Client()
	client.Timeout = 2 * time.Second
	response, err := client.Get(server.URL + "/caminho-inexistente")
	if err != nil {
		t.Fatalf("consultar caminho inexistente: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Errorf("status recebido = %d; esperado = %d", response.StatusCode, http.StatusNotFound)
	}
}
