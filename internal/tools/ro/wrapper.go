// Package ro adapts typed handler functions to net/http with JSON bodies and the protocol error envelope.
package ro

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"

	"cameo/internal/config"
	"cameo/internal/protocol"
	"cameo/server/services/request_context"
)

const maxBodySize = 1 << 20

type Validatable interface {
	Validate() error
}

type options struct {
	status int
}

type Option func(*options)

// WithStatus overrides the success status of a response with a body.
func WithStatus(status int) Option {
	return func(o *options) {
		o.status = status
	}
}

func resolveStatus(o []Option) int {
	resolved := options{status: http.StatusOK}
	for _, apply := range o {
		apply(&resolved)
	}
	return resolved.status
}

// Handle decodes and validates R, calls the handler and writes S (200 by default, 204 when nil).
func Handle[R, S any](handler func(context.Context, *R) (*S, error), o ...Option) http.HandlerFunc {
	status := resolveStatus(o)
	return func(w http.ResponseWriter, r *http.Request) {
		request := new(R)
		if err := decode(w, r, request); err != nil {
			WriteError(w, r, err)
			return
		}

		result, err := handler(r.Context(), request)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		if result == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		WriteJSON(w, status, result)
	}
}

// HandleIn decodes and validates R, calls the handler and responds 204.
func HandleIn[R any](handler func(context.Context, *R) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := new(R)
		if err := decode(w, r, request); err != nil {
			WriteError(w, r, err)
			return
		}

		if err := handler(r.Context(), request); err != nil {
			WriteError(w, r, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleOut calls the handler and writes S (200 by default, 204 when nil).
func HandleOut[S any](handler func(context.Context) (*S, error), o ...Option) http.HandlerFunc {
	status := resolveStatus(o)
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := handler(r.Context())
		if err != nil {
			WriteError(w, r, err)
			return
		}
		if result == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		WriteJSON(w, status, result)
	}
}

// HandleNone calls the handler and responds 204.
func HandleNone(handler func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(r.Context()); err != nil {
			WriteError(w, r, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// decode fills request from the query string (GET, DELETE) or the JSON body (other methods), then validates it.
func decode(w http.ResponseWriter, r *http.Request, request any) error {
	switch r.Method {
	case http.MethodGet, http.MethodDelete, http.MethodHead:
		if err := decodeQuery(r.URL.Query(), request); err != nil {
			return err
		}
	default:
		err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodySize)).Decode(request)
		if err != nil && !errors.Is(err, io.EOF) {
			return protocol.ErrorResponse{
				Code:    protocol.InvalidRequest,
				Message: fmt.Sprintf("failed to parse request: %v", err),
			}
		}
	}

	if validatable, ok := request.(Validatable); ok {
		if err := validatable.Validate(); err != nil {
			return validationError(err)
		}
	}

	return nil
}

func validationError(err error) error {
	if _, ok := errors.AsType[protocol.ErrorResponse](err); ok {
		return err
	}
	if _, ok := errors.AsType[validation.InternalError](err); ok {
		return err
	}

	response := protocol.ErrorResponse{
		Code:    protocol.InvalidRequest,
		Message: fmt.Sprintf("validation failed: %v", err),
	}
	if fields, ok := errors.AsType[validation.Errors](err); ok {
		messages := make(map[string]string, len(fields))
		for name, fieldErr := range fields {
			messages[name] = fieldErr.Error()
		}
		response.Meta = map[string]any{"fields": messages}
	}
	return response
}

// WriteJSON writes v as a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes err as a protocol.ErrorResponse. Errors that are not an ErrorResponse become 500 internal_error.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	logger := request_context.Logger(r.Context())

	if response, ok := errors.AsType[protocol.ErrorResponse](err); ok {
		logger.Debug().Msg(response.Error())
		WriteJSON(w, protocol.HTTPStatus(response.Code), response)
		return
	}

	if errors.Is(err, context.Canceled) {
		logger.Debug().Err(err).Msg("request canceled")
	} else {
		logger.Error().Err(err).Msg("internal server error")
	}

	response := protocol.ErrorResponse{
		Code:    protocol.InternalError,
		Message: "internal server error",
		Meta:    map[string]any{"request_id": request_context.RequestID(r.Context()).String()},
	}
	//goland:noinspection GoBoolExpressions
	if config.Version == "local" {
		response.Message = err.Error()
	}
	WriteJSON(w, http.StatusInternalServerError, response)
}

// PathUUID parses the named chi path parameter. A malformed ID is reported as not_found.
func PathUUID(ctx context.Context, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParamFromCtx(ctx, name))
	if err != nil {
		return uuid.Nil, protocol.ErrorResponse{
			Code:    protocol.NotFound,
			Message: "not found",
		}
	}
	return id, nil
}
