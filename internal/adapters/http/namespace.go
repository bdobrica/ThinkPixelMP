package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/namespace"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

type NamespaceUseCases interface {
	Create(context.Context, identity.Identity, string, publication.CreateNamespace) (namespace.Namespace, error)
	Get(context.Context, identity.Identity, shared.UUID) (namespace.Namespace, error)
	List(context.Context, identity.Identity, int, string) (publication.NamespacePage, error)
}

// NewNamespaceHandler exposes the Namespace create/read/list contract. It
// authenticates every operation; application authorization remains in the use case.
func NewNamespaceHandler(useCases NamespaceUseCases, authenticator identity.Authenticator) (http.Handler, error) {
	if useCases == nil || authenticator == nil {
		return nil, errors.New("namespace HTTP handler: use cases and authenticator are required")
	}
	handler := &namespaceHandler{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/namespaces", handler.collection)
	mux.HandleFunc("/v1/namespaces/{namespace_id}", handler.member)
	return namespaceRoutes{mux: mux}, nil
}

type namespaceRoutes struct{ mux *http.ServeMux }

func (routes namespaceRoutes) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	_, pattern := routes.mux.Handler(request)
	if pattern == "" {
		writeProblem(writer, http.StatusNotFound, "not_found", "route_not_found", "Not Found", requestID(request.Context()))
		return
	}
	routes.mux.ServeHTTP(writer, request)
}

type namespaceHandler struct {
	useCases      NamespaceUseCases
	authenticator identity.Authenticator
}

func (handler *namespaceHandler) collection(writer http.ResponseWriter, request *http.Request) {
	actor, err := authenticate(request, handler.authenticator)
	if err != nil {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		WriteError(writer, request, err)
		return
	}
	switch request.Method {
	case http.MethodPost:
		handler.create(writer, request, actor)
	case http.MethodGet:
		handler.list(writer, request, actor)
	default:
		writeProblem(writer, http.StatusMethodNotAllowed, "invalid", "method_not_allowed", "Method Not Allowed", requestID(request.Context()))
	}
}

func (handler *namespaceHandler) member(writer http.ResponseWriter, request *http.Request) {
	actor, err := authenticate(request, handler.authenticator)
	if err != nil {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		WriteError(writer, request, err)
		return
	}
	if request.Method != http.MethodGet {
		writeProblem(writer, http.StatusMethodNotAllowed, "invalid", "method_not_allowed", "Method Not Allowed", requestID(request.Context()))
		return
	}
	identifier, err := shared.ParseUUID(request.PathValue("namespace_id"))
	if err != nil {
		WriteError(writer, request, httpTyped(shared.ErrorInvalid, "namespace.invalid_id"))
		return
	}
	value, err := handler.useCases.Get(request.Context(), actor, identifier)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	writeNamespace(writer, http.StatusOK, value)
}

func (handler *namespaceHandler) create(writer http.ResponseWriter, request *http.Request, actor identity.Identity) {
	request, err := withAuditIdentity(request, actor)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	key := request.Header.Get(idempotencyKeyHeader)
	if len(request.Header.Values(idempotencyKeyHeader)) != 1 || key == "" {
		WriteError(writer, request, httpTyped(shared.ErrorInvalid, "namespace.idempotency_key_required"))
		return
	}
	var command publication.CreateNamespace
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&command); err != nil {
		handler.writeDecodeError(writer, request, err)
		return
	}
	if err := requireEOF(decoder); err != nil {
		handler.writeDecodeError(writer, request, err)
		return
	}
	value, err := handler.useCases.Create(request.Context(), actor, key, command)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	writer.Header().Set("Location", "/v1/namespaces/"+value.ID().String())
	writeNamespace(writer, http.StatusCreated, value)
}

func (handler *namespaceHandler) list(writer http.ResponseWriter, request *http.Request, actor identity.Identity) {
	query := request.URL.Query()
	for key, values := range query {
		if (key != "cursor" && key != "page_size") || len(values) != 1 {
			WriteError(writer, request, httpTyped(shared.ErrorInvalid, "namespace.invalid_query"))
			return
		}
	}
	pageSize := publication.DefaultNamespacePageSize
	if raw := query.Get("page_size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			WriteError(writer, request, httpTyped(shared.ErrorInvalid, "namespace.invalid_page_size"))
			return
		}
		pageSize = parsed
	}
	page, err := handler.useCases.List(request.Context(), actor, pageSize, query.Get("cursor"))
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	response := namespacePageResponse{Items: make([]namespaceResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, value := range page.Items {
		response.Items = append(response.Items, namespaceRepresentation(value))
	}
	writeJSON(writer, http.StatusOK, response)
}

func (handler *namespaceHandler) writeDecodeError(writer http.ResponseWriter, request *http.Request, err error) {
	var maximum *http.MaxBytesError
	if errors.As(err, &maximum) {
		writeProblem(writer, http.StatusRequestEntityTooLarge, "invalid", "request_body_too_large", "Content Too Large", requestID(request.Context()))
		return
	}
	WriteError(writer, request, httpTyped(shared.ErrorInvalid, "namespace.invalid_request"))
}

type namespaceResponse struct {
	ID               string `json:"id"`
	TenantID         string `json:"tenant_id"`
	Path             string `json:"path"`
	OwnerPublisherID string `json:"owner_publisher_id"`
	CreatedAt        string `json:"created_at"`
}

type namespacePageResponse struct {
	Items      []namespaceResponse `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

func namespaceRepresentation(value namespace.Namespace) namespaceResponse {
	return namespaceResponse{ID: value.ID().String(), TenantID: value.TenantID().String(), Path: value.Path(),
		OwnerPublisherID: value.OwnerPublisherID().String(), CreatedAt: value.CreatedAt().UTC().Format("2006-01-02T15:04:05.000000000Z")}
}

func writeNamespace(writer http.ResponseWriter, status int, value namespace.Namespace) {
	writeJSON(writer, status, namespaceRepresentation(value))
}
