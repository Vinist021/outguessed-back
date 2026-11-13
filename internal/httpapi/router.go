package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter cria o roteador HTTP da aplicação, ainda sem rotas cadastradas.
func NewRouter() http.Handler {
	return chi.NewRouter()
}
