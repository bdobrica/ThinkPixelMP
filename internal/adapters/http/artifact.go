package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/artifact"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
)

type ArtifactUseCases interface {
	Create(context.Context, identity.Identity, string, publication.CreateArtifact) (artifact.Artifact, error)
	Get(context.Context, identity.Identity, shared.UUID) (artifact.Artifact, error)
	List(context.Context, identity.Identity, string, int, string) (publication.ArtifactPage, error)
}

// NewArtifactHandler exposes the logical Artifact create/read/list contract.
func NewArtifactHandler(useCases ArtifactUseCases, authenticator identity.Authenticator) (http.Handler, error) {
	if useCases == nil || authenticator == nil {
		return nil, errors.New("artifact HTTP handler: use cases and authenticator are required")
	}
	handler := &artifactHandler{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/artifacts", handler.collection)
	mux.HandleFunc("/v1/artifacts/{artifact_id}", handler.member)
	return artifactRoutes{mux: mux}, nil
}

type artifactRoutes struct{ mux *http.ServeMux }

func (routes artifactRoutes) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	_, pattern := routes.mux.Handler(request)
	if pattern == "" {
		writeProblem(writer, http.StatusNotFound, "not_found", "route_not_found", "Not Found", requestID(request.Context()))
		return
	}
	routes.mux.ServeHTTP(writer, request)
}

type artifactHandler struct {
	useCases      ArtifactUseCases
	authenticator identity.Authenticator
}

func (handler *artifactHandler) collection(writer http.ResponseWriter, request *http.Request) {
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

func (handler *artifactHandler) member(writer http.ResponseWriter, request *http.Request) {
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
	identifier, err := shared.ParseUUID(request.PathValue("artifact_id"))
	if err != nil {
		WriteError(writer, request, httpTyped(shared.ErrorInvalid, "artifact.invalid_id"))
		return
	}
	value, err := handler.useCases.Get(request.Context(), actor, identifier)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	writeArtifact(writer, http.StatusOK, value)
}

func (handler *artifactHandler) create(writer http.ResponseWriter, request *http.Request, actor identity.Identity) {
	request, err := withAuditIdentity(request, actor)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	key := request.Header.Get(idempotencyKeyHeader)
	if len(request.Header.Values(idempotencyKeyHeader)) != 1 || key == "" {
		WriteError(writer, request, httpTyped(shared.ErrorInvalid, "artifact.idempotency_key_required"))
		return
	}
	var command publication.CreateArtifact
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
	writer.Header().Set("Location", "/v1/artifacts/"+value.ID().String())
	writeArtifact(writer, http.StatusCreated, value)
}

func (handler *artifactHandler) list(writer http.ResponseWriter, request *http.Request, actor identity.Identity) {
	query := request.URL.Query()
	for key, values := range query {
		if (key != "cursor" && key != "page_size" && key != "query") || len(values) != 1 {
			WriteError(writer, request, httpTyped(shared.ErrorInvalid, "artifact.invalid_query"))
			return
		}
	}
	pageSize := publication.DefaultArtifactPageSize
	if raw := query.Get("page_size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			WriteError(writer, request, httpTyped(shared.ErrorInvalid, "artifact.invalid_page_size"))
			return
		}
		pageSize = parsed
	}
	page, err := handler.useCases.List(request.Context(), actor, query.Get("query"), pageSize, query.Get("cursor"))
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	response := artifactPageResponse{Items: make([]artifactResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, value := range page.Items {
		response.Items = append(response.Items, artifactRepresentation(value))
	}
	writeJSON(writer, http.StatusOK, response)
}

func (handler *artifactHandler) writeDecodeError(writer http.ResponseWriter, request *http.Request, err error) {
	var maximum *http.MaxBytesError
	if errors.As(err, &maximum) {
		writeProblem(writer, http.StatusRequestEntityTooLarge, "invalid", "request_body_too_large", "Content Too Large", requestID(request.Context()))
		return
	}
	WriteError(writer, request, httpTyped(shared.ErrorInvalid, "artifact.invalid_request"))
}

type artifactResponse struct {
	ID          string            `json:"id"`
	TenantID    string            `json:"tenant_id"`
	NamespaceID string            `json:"namespace_id"`
	Identity    string            `json:"identity"`
	Name        string            `json:"name"`
	Kind        string            `json:"kind"`
	DisplayName string            `json:"display_name,omitempty"`
	Description string            `json:"description,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   string            `json:"created_at"`
}

type artifactPageResponse struct {
	Items      []artifactResponse `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

func artifactRepresentation(value artifact.Artifact) artifactResponse {
	return artifactResponse{ID: value.ID().String(), TenantID: value.TenantID().String(), NamespaceID: value.NamespaceID().String(),
		Identity: value.Identity().String(), Name: value.Name(), Kind: string(value.Kind()), DisplayName: value.DisplayName(),
		Description: value.Description(), Homepage: value.Homepage(), Repository: value.Repository(), Labels: value.Labels(),
		CreatedAt: value.CreatedAt().UTC().Format("2006-01-02T15:04:05.000000000Z")}
}

func writeArtifact(writer http.ResponseWriter, status int, value artifact.Artifact) {
	writeJSON(writer, status, artifactRepresentation(value))
}
