package httpapi

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/Vinist021/outguessed-back/internal/health"
)

func TestOpenAPIResponses(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromFile("../../docs/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := spec.Validate(t.Context()); err != nil {
		t.Fatalf("OpenAPI inválido: %v", err)
	}

	for _, scenario := range []struct {
		name, path, method string
		status             int
		component          string
		check              func(context.Context) error
	}{
		{name: "vivacidade", path: livePath, method: http.MethodGet, status: 200},
		{name: "prontidão", path: readyPath, method: http.MethodGet, status: 200},
		{name: "caminho inexistente", path: "/inexistente", method: http.MethodGet, status: 404, component: "NotFound"},
		{name: "método incompatível", path: livePath, method: http.MethodPost, status: 405, component: "MethodNotAllowed"},
		{name: "erro interno", path: readyPath, method: http.MethodGet, status: 500, check: func(context.Context) error {
			panic("falha de teste")
		}},
		{name: "serviço indisponível", path: readyPath, method: http.MethodGet, status: 503, check: func(context.Context) error {
			return health.ErrSchemaMissing
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			check := scenario.check
			if check == nil {
				check = func(context.Context) error { return nil }
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(scenario.method, scenario.path, nil)
			NewRouter(check, discardLogger(), nil).ServeHTTP(response, request)
			if response.Code != scenario.status {
				t.Fatalf("status recebido = %d; esperado = %d", response.Code, scenario.status)
			}

			var documented *openapi3.ResponseRef
			if scenario.component != "" {
				documented = spec.Components.Responses[scenario.component]
			} else {
				path := spec.Paths.Find(scenario.path)
				if path == nil || path.Get == nil {
					t.Fatalf("operação GET %s ausente no OpenAPI", scenario.path)
				}
				documented = path.Get.Responses.Status(response.Code)
			}
			if documented == nil || documented.Value == nil {
				t.Fatalf("resposta %d ausente no OpenAPI", response.Code)
			}
			validateResponseSchema(t, response, documented.Value)
		})
	}
}

func validateResponseSchema(t *testing.T, actual *httptest.ResponseRecorder, documented *openapi3.Response) {
	t.Helper()
	contentType := actual.Header().Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("Content-Type inválido: %q", contentType)
	}
	content := documented.Content[mediaType]
	if content == nil || content.Schema == nil {
		t.Fatalf("schema de %s ausente no OpenAPI", mediaType)
	}
	var body any
	if err := json.Unmarshal(actual.Body.Bytes(), &body); err != nil {
		t.Fatalf("corpo JSON inválido: %v", err)
	}
	if err := content.Schema.Value.VisitJSON(body); err != nil {
		t.Errorf("corpo incompatível com o OpenAPI: %v", err)
	}
	for name, reference := range documented.Headers {
		header := reference.Value
		values := actual.Header().Values(name)
		if header.Required && len(values) == 0 {
			t.Errorf("cabeçalho obrigatório ausente: %s", name)
		}
		if header.Schema != nil {
			for _, value := range values {
				if err := header.Schema.Value.VisitJSON(value); err != nil {
					t.Errorf("cabeçalho %s incompatível com o OpenAPI: %v", name, err)
				}
			}
		}
	}
}
