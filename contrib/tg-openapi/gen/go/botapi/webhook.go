// Package botapi provides types and handlers for the Telegram Bot API webhook.
package botapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/oapi-codegen/runtime"
)

var (
	ErrUnexpectedResponse = errors.New("unexpected response type")
	ErrUnexpectedRequest  = errors.New("unexpected request type")
)

// SendUpdateJSONRequestBody defines body for SendUpdate for application/json ContentType.
type SendUpdateJSONRequestBody Update

// SendUpdateParams defines parameters for SendUpdate.
//
//nolint:tagliatelle
type SendUpdateParams struct {
	// Токен secret token for webhook authentication
	Token *string `json:"X-Telegram-Bot-Api-Secret-Token,omitempty"`
}

// WebhookInterface represents all server handlers.
type WebhookInterface interface {
	// (POST /)
	SendUpdate(w http.ResponseWriter, r *http.Request, params SendUpdateParams)
}

// WebhookInterfaceWrapper converts contexts to parameters.
type WebhookInterfaceWrapper struct {
	Handler            WebhookInterface
	ErrorHandlerFunc   func(w http.ResponseWriter, r *http.Request, err error)
	HandlerMiddlewares []MiddlewareFunc
}

// SendUpdate operation middleware
func (siw *WebhookInterfaceWrapper) SendUpdate(w http.ResponseWriter, r *http.Request) {
	params, ok := siw.parseSendUpdateParams(w, r)
	if !ok {
		return
	}

	handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		siw.Handler.SendUpdate(w, r, params)
	}))

	for _, middleware := range siw.HandlerMiddlewares {
		handler = middleware(handler)
	}

	handler.ServeHTTP(w, r)
}

func (siw *WebhookInterfaceWrapper) parseSendUpdateParams(
	w http.ResponseWriter, r *http.Request,
) (SendUpdateParams, bool) {
	var params SendUpdateParams

	headers := r.Header

	// ------------- Optional header parameter "X-Telegram-Bot-Api-Secret-Token" -------------
	secretTokenKey := http.CanonicalHeaderKey("X-Telegram-Bot-Api-Secret-Token")
	valueList, found := headers[secretTokenKey]

	if !found {
		return params, true
	}

	if len(valueList) != 1 {
		siw.ErrorHandlerFunc(w, r, &TooManyValuesForParamError{
			ParamName: "X-Telegram-Bot-Api-Secret-Token", Count: len(valueList),
		})

		return params, false
	}

	token, ok := siw.bindSecretToken(w, r, valueList[0])
	if !ok {
		return params, false
	}

	params.Token = &token

	return params, true
}

func (siw *WebhookInterfaceWrapper) bindSecretToken(
	w http.ResponseWriter, r *http.Request, value string,
) (string, bool) {
	var token string

	err := runtime.BindStyledParameterWithOptions(
		"simple", "X-Telegram-Bot-Api-Secret-Token", value,
		&token, runtime.BindStyledParameterOptions{
			ParamLocation: runtime.ParamLocationHeader,
			Explode:       false,
			Required:      false,
			Type:          "string",
			Format:        "",
		})
	if err != nil {
		siw.ErrorHandlerFunc(w, r, &InvalidParamFormatError{
			ParamName: "X-Telegram-Bot-Api-Secret-Token", Err: err,
		})

		return "", false
	}

	return token, true
}

// WebhookHandler creates http.Handler with routing matching OpenAPI spec.
func WebhookHandler(si WebhookInterface) http.Handler {
	return WebhookHandlerWithOptions(si, WebhookServerOptions{
		BaseRouter:       nil,
		ErrorHandlerFunc: nil,
		BaseURL:          "",
		Middlewares:      nil,
	})
}

type WebhookServerOptions struct {
	BaseRouter       ServeMux
	ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
	BaseURL          string
	Middlewares      []MiddlewareFunc
}

// WebhookHandlerFromMux creates http.Handler with routing matching OpenAPI spec
// based on the provided mux.
func WebhookHandlerFromMux(si WebhookInterface, m ServeMux) http.Handler {
	return WebhookHandlerWithOptions(si, WebhookServerOptions{
		BaseRouter:       m,
		ErrorHandlerFunc: nil,
		BaseURL:          "",
		Middlewares:      nil,
	})
}

func WebhookHandlerFromMuxWithBaseURL(
	si WebhookInterface, m ServeMux, baseURL string,
) http.Handler {
	return WebhookHandlerWithOptions(si, WebhookServerOptions{
		BaseURL:          baseURL,
		BaseRouter:       m,
		ErrorHandlerFunc: nil,
		Middlewares:      nil,
	})
}

// WebhookHandlerWithOptions creates http.Handler with additional options
func WebhookHandlerWithOptions(
	webhook WebhookInterface, options WebhookServerOptions,
) http.Handler {
	mux := options.BaseRouter

	if mux == nil {
		mux = http.NewServeMux()
	}

	if options.ErrorHandlerFunc == nil {
		options.ErrorHandlerFunc = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}

	wrapper := WebhookInterfaceWrapper{
		Handler:            webhook,
		HandlerMiddlewares: options.Middlewares,
		ErrorHandlerFunc:   options.ErrorHandlerFunc,
	}

	mux.HandleFunc("POST "+options.BaseURL+"/{$}", wrapper.SendUpdate)

	return mux
}

type SendUpdateRequestObject struct {
	Params SendUpdateParams
	Body   *SendUpdateJSONRequestBody
}

type SendUpdateResponseObject interface {
	VisitSendUpdateResponse(w http.ResponseWriter) error
}

type SendUpdate204Response struct{}

func (response SendUpdate204Response) VisitSendUpdateResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)

	return json.NewEncoder(w).Encode(response)
}

// StrictWebhookInterface represents all server handlers.
type StrictWebhookInterface interface {
	// (POST /)
	SendUpdate(
		ctx context.Context, request SendUpdateRequestObject,
	) (SendUpdateResponseObject, error)
}

func NewStrictWebhookHandler(
	ssi StrictWebhookInterface, middlewares []StrictMiddlewareFunc,
) WebhookInterface {
	return &strictWebhookHandler{
		ssi: ssi, middlewares: middlewares, options: StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				http.Error(w, err.Error(), http.StatusBadRequest)
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			},
		},
	}
}

func NewStrictWebhookHandlerWithOptions(
	ssi StrictWebhookInterface,
	middlewares []StrictMiddlewareFunc,
	options StrictHTTPServerOptions,
) WebhookInterface {
	return &strictWebhookHandler{ssi: ssi, middlewares: middlewares, options: options}
}

type strictWebhookHandler struct {
	ssi         StrictWebhookInterface
	options     StrictHTTPServerOptions
	middlewares []StrictMiddlewareFunc
}

// SendUpdate operation middleware
func (sh *strictWebhookHandler) SendUpdate(
	w http.ResponseWriter, r *http.Request, params SendUpdateParams,
) {
	request := SendUpdateRequestObject{
		Params: params,
		Body:   nil,
	}

	var body SendUpdateJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sh.options.RequestErrorHandlerFunc(w, r, fmt.Errorf("can't decode JSON body: %w", err))

		return
	}

	request.Body = &body

	handler := sh.buildHandler()

	for _, middleware := range sh.middlewares {
		handler = middleware(handler, "SendUpdate")
	}

	sh.handleResponse(w, r, handler, request)
}

func (sh *strictWebhookHandler) buildHandler() func(
	context.Context, http.ResponseWriter, *http.Request, any,
) (any, error) {
	return func(
		ctx context.Context, _ http.ResponseWriter, _ *http.Request, req interface{},
	) (any, error) {
		typedReq, ok := req.(SendUpdateRequestObject)
		if !ok {
			return nil, fmt.Errorf("%w: %T", ErrUnexpectedRequest, req)
		}

		res, err := sh.ssi.SendUpdate(ctx, typedReq)
		if err != nil {
			return nil, fmt.Errorf("send update: %w", err)
		}

		return res, nil
	}
}

func (sh *strictWebhookHandler) handleResponse(
	w http.ResponseWriter,
	r *http.Request,
	handler func(context.Context, http.ResponseWriter, *http.Request, any) (any, error),
	request SendUpdateRequestObject,
) {
	response, err := handler(r.Context(), w, r, request)
	if err != nil {
		sh.options.ResponseErrorHandlerFunc(w, r, err)

		return
	}

	validResponse, ok := response.(SendUpdateResponseObject)
	if !ok {
		if response != nil {
			sh.options.ResponseErrorHandlerFunc(
				w, r, fmt.Errorf("%w: %T", ErrUnexpectedResponse, response),
			)
		}

		return
	}

	if err := validResponse.VisitSendUpdateResponse(w); err != nil {
		sh.options.ResponseErrorHandlerFunc(w, r, err)
	}
}
