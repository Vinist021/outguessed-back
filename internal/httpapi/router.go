package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

const (
	livePath  = "/health/live"
	readyPath = "/health/ready"
)

func NewRouter(checkReadiness func(context.Context) error, logger *slog.Logger, allowedOrigins []string) http.Handler {
	router := chi.NewRouter()
	allowsGet := func(r *http.Request) bool {
		return router.Match(chi.NewRouteContext(), http.MethodGet, r.URL.EscapedPath())
	}
	router.Use(observeRequests(logger), cors(allowedOrigins, allowsGet))
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeProblem(w, r, notFound)
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		// O Chi delega a escrita de Allow ao handler personalizado e também
		// o chama para métodos desconhecidos, mesmo antes de resolver o caminho.
		if !allowsGet(r) {
			writeProblem(w, r, notFound)
			return
		}
		w.Header().Set("Allow", http.MethodGet)
		writeProblem(w, r, methodNotAllowed)
	})
	router.Get(livePath, func(w http.ResponseWriter, r *http.Request) {
		writeHealthy(w)
	})
	router.Get(readyPath, func(w http.ResponseWriter, r *http.Request) {
		if err := checkReadiness(r.Context()); err != nil {
			logger.WarnContext(r.Context(), "Verificação HTTP de prontidão falhou",
				"request_id", requestID(r.Context()), "cause", readinessFailure(err))
			writeProblem(w, r, serviceUnavailable)
			return
		}
		writeHealthy(w)
	})
	return router
}
