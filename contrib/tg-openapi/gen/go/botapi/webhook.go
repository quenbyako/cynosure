package botapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/oapi-codegen/runtime"
)

// SendUpdateJSONRequestBody defines body for SendUpdate for application/json ContentType.
type SendUpdateJSONRequestBody Update

// SendUpdateParams defines parameters for SendUpdate.
type SendUpdateParams struct {
	// XTelegramBotApiSecretToken secret token for webhook authentication
	XTelegramBotApiSecretToken *string `json:"X-Telegram-Bot-Api-Secret-Token,omitempty"`
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
	// Parameter object where we will unmarshal all parameters from the context
	var (
		params SendUpdateParams
		err    error
	)

	headers := r.Header

	// ------------- Optional header parameter "X-Telegram-Bot-Api-Secret-Token" -------------
	if valueList, found := headers[http.CanonicalHeaderKey("X-Telegram-Bot-Api-Secret-Token")]; found {
		var XTelegramBotApiSecretToken string

		n := len(valueList)
		if n != 1 {
			siw.ErrorHandlerFunc(w, r, &TooManyValuesForParamError{ParamName: "X-Telegram-Bot-Api-Secret-Token", Count: n})
			return
		}

		err = runtime.BindStyledParameterWithOptions("simple", "X-Telegram-Bot-Api-Secret-Token", valueList[0], &XTelegramBotApiSecretToken, runtime.BindStyledParameterOptions{ParamLocation: runtime.ParamLocationHeader, Explode: false, Required: false})
		if err != nil {
			siw.ErrorHandlerFunc(w, r, &InvalidParamFormatError{ParamName: "X-Telegram-Bot-Api-Secret-Token", Err: err})
			return
		}

		params.XTelegramBotApiSecretToken = &XTelegramBotApiSecretToken
	}

	handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		siw.Handler.SendUpdate(w, r, params)
	}))

	for _, middleware := range siw.HandlerMiddlewares {
		handler = middleware(handler)
	}

	handler.ServeHTTP(w, r)
}

// WebhookHandler creates http.Handler with routing matching OpenAPI spec.
func WebhookHandler(si WebhookInterface) http.Handler {
	return WebhookHandlerWithOptions(si, WebhookServerOptions{})
}

type WebhookServerOptions struct {
	BaseRouter       ServeMux
	ErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
	BaseURL          string
	Middlewares      []MiddlewareFunc
}

// WebhookHandlerFromMux creates http.Handler with routing matching OpenAPI spec based on the provided mux.
func WebhookHandlerFromMux(si WebhookInterface, m ServeMux) http.Handler {
	return WebhookHandlerWithOptions(si, WebhookServerOptions{
		BaseRouter: m,
	})
}

func WebhookHandlerFromMuxWithBaseURL(si WebhookInterface, m ServeMux, baseURL string) http.Handler {
	return WebhookHandlerWithOptions(si, WebhookServerOptions{
		BaseURL:    baseURL,
		BaseRouter: m,
	})
}

// WebhookHandlerWithOptions creates http.Handler with additional options
func WebhookHandlerWithOptions(si WebhookInterface, options WebhookServerOptions) http.Handler {
	m := options.BaseRouter

	if m == nil {
		m = http.NewServeMux()
	}

	if options.ErrorHandlerFunc == nil {
		options.ErrorHandlerFunc = func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}

	wrapper := WebhookInterfaceWrapper{
		Handler:            si,
		HandlerMiddlewares: options.Middlewares,
		ErrorHandlerFunc:   options.ErrorHandlerFunc,
	}

	m.HandleFunc("POST "+options.BaseURL+"/{$}", wrapper.SendUpdate)

	return m
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
	SendUpdate(ctx context.Context, request SendUpdateRequestObject) (SendUpdateResponseObject, error)
}

func NewStrictWebhookHandler(ssi StrictWebhookInterface, middlewares []StrictMiddlewareFunc) WebhookInterface {
	return &strictWebhookHandler{ssi: ssi, middlewares: middlewares, options: StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		},
	}}
}

func NewStrictWebhookHandlerWithOptions(ssi StrictWebhookInterface, middlewares []StrictMiddlewareFunc, options StrictHTTPServerOptions) WebhookInterface {
	return &strictWebhookHandler{ssi: ssi, middlewares: middlewares, options: options}
}

type strictWebhookHandler struct {
	ssi         StrictWebhookInterface
	options     StrictHTTPServerOptions
	middlewares []StrictMiddlewareFunc
}

// SendUpdate operation middleware
func (sh *strictWebhookHandler) SendUpdate(w http.ResponseWriter, r *http.Request, params SendUpdateParams) {
	var request SendUpdateRequestObject

	request.Params = params

	var body SendUpdateJSONRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sh.options.RequestErrorHandlerFunc(w, r, fmt.Errorf("can't decode JSON body: %w", err))
		return
	}

	request.Body = &body

	handler := func(ctx context.Context, w http.ResponseWriter, r *http.Request, request interface{}) (interface{}, error) {
		return sh.ssi.SendUpdate(ctx, request.(SendUpdateRequestObject))
	}
	for _, middleware := range sh.middlewares {
		handler = middleware(handler, "SendUpdate")
	}

	response, err := handler(r.Context(), w, r, request)
	if err != nil {
		sh.options.ResponseErrorHandlerFunc(w, r, err)
	} else if validResponse, ok := response.(SendUpdateResponseObject); ok {
		if err := validResponse.VisitSendUpdateResponse(w); err != nil {
			sh.options.ResponseErrorHandlerFunc(w, r, err)
		}
	} else if response != nil {
		sh.options.ResponseErrorHandlerFunc(w, r, fmt.Errorf("unexpected response type: %T", response))
	}
}
