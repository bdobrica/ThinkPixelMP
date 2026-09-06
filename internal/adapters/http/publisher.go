package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bdobrica/ThinkPixelMP/internal/app/publication"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/audit"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/publisher"
	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
	"github.com/bdobrica/ThinkPixelMP/internal/ports/identity"
	"github.com/bdobrica/ThinkPixelMP/internal/telemetry/logging"
)

const idempotencyKeyHeader = "Idempotency-Key"

type PublisherUseCases interface {
	Create(context.Context, identity.Identity, string, publication.CreatePublisher) (publisher.Publisher, error)
	Get(context.Context, identity.Identity, shared.UUID) (publisher.Publisher, error)
	List(context.Context, identity.Identity, int, string) (publication.PublisherPage, error)
}

// NewPublisherHandler exposes the Publisher create/read/list contract. It
// authenticates every operation; application authorization remains in the use case.
func NewPublisherHandler(useCases PublisherUseCases, authenticator identity.Authenticator) (http.Handler, error) {
	if useCases == nil || authenticator == nil {
		return nil, errors.New("publisher HTTP handler: use cases and authenticator are required")
	}
	handler := &publisherHandler{useCases: useCases, authenticator: authenticator}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/publishers", handler.collection)
	mux.HandleFunc("/v1/publishers/{publisher_id}", handler.member)
	return publisherRoutes{mux: mux}, nil
}

type publisherRoutes struct{ mux *http.ServeMux }

func (routes publisherRoutes) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	_, pattern := routes.mux.Handler(request)
	if pattern == "" {
		writeProblem(writer, http.StatusNotFound, "not_found", "route_not_found", "Not Found", requestID(request.Context()))
		return
	}
	routes.mux.ServeHTTP(writer, request)
}

type publisherHandler struct {
	useCases      PublisherUseCases
	authenticator identity.Authenticator
}

func (handler *publisherHandler) collection(writer http.ResponseWriter, request *http.Request) {
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

func (handler *publisherHandler) member(writer http.ResponseWriter, request *http.Request) {
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
	identifier, err := shared.ParseUUID(request.PathValue("publisher_id"))
	if err != nil {
		WriteError(writer, request, httpTyped(shared.ErrorInvalid, "publisher.invalid_id"))
		return
	}
	value, err := handler.useCases.Get(request.Context(), actor, identifier)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	writePublisher(writer, http.StatusOK, value)
}

func (handler *publisherHandler) create(writer http.ResponseWriter, request *http.Request, actor identity.Identity) {
	request, err := withAuditIdentity(request, actor)
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	key := request.Header.Get(idempotencyKeyHeader)
	if len(request.Header.Values(idempotencyKeyHeader)) != 1 || key == "" {
		WriteError(writer, request, httpTyped(shared.ErrorInvalid, "publisher.idempotency_key_required"))
		return
	}
	var command publication.CreatePublisher
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
	writer.Header().Set("Location", "/v1/publishers/"+value.ID().String())
	writePublisher(writer, http.StatusCreated, value)
}

func withAuditIdentity(request *http.Request, mapped identity.Identity) (*http.Request, error) {
	correlation := logging.CorrelationFromContext(request.Context())
	var requestIdentifier *shared.UUID
	if correlation.RequestID != "" {
		parsed, err := shared.ParseUUID(correlation.RequestID)
		if err != nil {
			return request, httpTyped(shared.ErrorInternal, "audit.context_unavailable")
		}
		requestIdentifier = &parsed
	}
	actor, err := audit.NewActor(mapped.PrincipalID, requestIdentifier, correlation.TraceID)
	if err != nil {
		return request, httpTyped(shared.ErrorUnauthorized, "authorization.identity_required")
	}
	return request.WithContext(audit.WithActor(request.Context(), actor)), nil
}

func (handler *publisherHandler) list(writer http.ResponseWriter, request *http.Request, actor identity.Identity) {
	query := request.URL.Query()
	for key, values := range query {
		if (key != "cursor" && key != "page_size") || len(values) != 1 {
			WriteError(writer, request, httpTyped(shared.ErrorInvalid, "publisher.invalid_query"))
			return
		}
	}
	pageSize := publication.DefaultPublisherPageSize
	if raw := query.Get("page_size"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			WriteError(writer, request, httpTyped(shared.ErrorInvalid, "publisher.invalid_page_size"))
			return
		}
		pageSize = parsed
	}
	page, err := handler.useCases.List(request.Context(), actor, pageSize, query.Get("cursor"))
	if err != nil {
		WriteError(writer, request, err)
		return
	}
	response := publisherPageResponse{Items: make([]publisherResponse, 0, len(page.Items)), NextCursor: page.NextCursor}
	for _, value := range page.Items {
		response.Items = append(response.Items, publisherRepresentation(value))
	}
	writeJSON(writer, http.StatusOK, response)
}

func authenticate(request *http.Request, authenticator identity.Authenticator) (identity.Identity, error) {
	values := request.Header.Values("Authorization")
	credential := ""
	if len(values) > 1 {
		return identity.Identity{}, httpTyped(shared.ErrorUnauthorized, "identity.invalid_authorization")
	}
	if len(values) == 1 {
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return identity.Identity{}, httpTyped(shared.ErrorUnauthorized, "identity.invalid_authorization")
		}
		credential = parts[1]
	}
	return authenticator.Authenticate(request.Context(), credential)
}

func (handler *publisherHandler) writeDecodeError(writer http.ResponseWriter, request *http.Request, err error) {
	var maximum *http.MaxBytesError
	if errors.As(err, &maximum) {
		writeProblem(writer, http.StatusRequestEntityTooLarge, "invalid", "request_body_too_large", "Content Too Large", requestID(request.Context()))
		return
	}
	WriteError(writer, request, httpTyped(shared.ErrorInvalid, "publisher.invalid_request"))
}

func requireEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return errors.New("trailing JSON value")
}

type publisherResponse struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state"`
	CreatedAt   string `json:"created_at"`
}

type publisherPageResponse struct {
	Items      []publisherResponse `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}

func publisherRepresentation(value publisher.Publisher) publisherResponse {
	return publisherResponse{ID: value.ID().String(), TenantID: value.TenantID().String(), Slug: value.Slug(),
		DisplayName: value.DisplayName(), Description: value.Description(), State: string(value.State()),
		CreatedAt: value.CreatedAt().UTC().Format("2006-01-02T15:04:05.000000000Z")}
}

func writePublisher(writer http.ResponseWriter, status int, value publisher.Publisher) {
	writer.Header().Set("ETag", `"`+strconv.FormatInt(value.StateVersion(), 10)+`"`)
	writeJSON(writer, status, publisherRepresentation(value))
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func httpTyped(class shared.ErrorClass, code string) error {
	reason, _ := shared.NewReasonCode(code)
	return shared.NewTypedError(class, reason)
}
