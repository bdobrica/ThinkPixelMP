package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bdobrica/ThinkPixelMP/internal/domain/shared"
)

type problem struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

// WriteError maps safe typed domain errors to RFC 7807. Unknown errors are
// deliberately collapsed to a stable internal problem without their text.
func WriteError(writer http.ResponseWriter, request *http.Request, err error) {
	status, class, code, title := http.StatusInternalServerError, "internal", "internal_error", "Internal Server Error"
	var typed *shared.TypedError
	if errors.As(err, &typed) {
		class, code = string(typed.Class()), typed.Code().String()
		switch typed.Class() {
		case shared.ErrorInvalid:
			status, title = http.StatusBadRequest, "Bad Request"
		case shared.ErrorNotFound:
			status, title = http.StatusNotFound, "Not Found"
		case shared.ErrorConflict:
			status, title = http.StatusConflict, "Conflict"
		case shared.ErrorUnauthorized:
			status, title = http.StatusUnauthorized, "Unauthorized"
		case shared.ErrorForbidden:
			status, title = http.StatusForbidden, "Forbidden"
		case shared.ErrorUnavailable:
			status, title = http.StatusServiceUnavailable, "Service Unavailable"
		case shared.ErrorInternal:
			status, title = http.StatusInternalServerError, "Internal Server Error"
		}
	}
	writeProblem(writer, status, class, code, title, requestID(request.Context()))
}

func writeProblem(writer http.ResponseWriter, status int, class, code, title, requestID string) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(problem{Type: "https://thinkpixel.dev/problems/" + class + "/" + code, Title: title, Status: status, Code: code, RequestID: requestID})
}

func writeHealth(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(writer).Encode(struct {
		Status string `json:"status"`
	}{Status: "ok"})
}
