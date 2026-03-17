package messages

import (
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/url"
)

type ChatContent interface {
	URL() *url.URL
	Type() (mediaType string, params map[string]string)
	Extra() map[string]json.RawMessage

	_ChatContent()
}

var (
	_ ChatContent = (*ChatContentText)(nil)
	_ ChatContent = (*ChatContentAudioURL)(nil)
	_ ChatContent = (*ChatContentVideoURL)(nil)
	_ ChatContent = (*ChatContentFileURL)(nil)
	_ ChatContent = (*ChatContentImageURL)(nil)
)

type ChatContentText struct {
	extra map[string]json.RawMessage
	text  string
}

func (c *ChatContentText) _ChatContent() {}

func NewChatMessageText(text string, extra map[string]json.RawMessage) *ChatContentText {
	return &ChatContentText{
		text:  text,
		extra: extra,
	}
}

func (c *ChatContentText) Type() (mediaType string, params map[string]string) {
	return "text/plain", nil
}

// URL of content.
func (c *ChatContentText) URL() *url.URL {
	//nolint:exhaustruct // intentional data URL
	return &url.URL{
		Scheme: "data",
		Opaque: "text/plain;charset=utf-8;base64," +
			base64.StdEncoding.EncodeToString([]byte(c.text)),
	}
}
func (c *ChatContentText) Extra() map[string]json.RawMessage { return c.extra }

func (c *ChatContentText) Text() string { return c.text }

type ChatContentAudioURL struct {
	extra    map[string]json.RawMessage
	mimeType string
	address  url.URL
}

func (c *ChatContentAudioURL) _ChatContent() {}

func (c *ChatContentAudioURL) Type() (mediaType string, params map[string]string) {
	//nolint:errcheck // handled via default values
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentAudioURL) URL() *url.URL                     { return clone(&c.address) }
func (c *ChatContentAudioURL) Extra() map[string]json.RawMessage { return c.extra }

type ChatContentVideoURL struct {
	extra    map[string]json.RawMessage
	mimeType string
	address  url.URL
}

func (c *ChatContentVideoURL) _ChatContent() {}

func (c *ChatContentVideoURL) Type() (mediaType string, params map[string]string) {
	//nolint:errcheck // handled via default values
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentVideoURL) URL() *url.URL                     { return clone(&c.address) }
func (c *ChatContentVideoURL) Extra() map[string]json.RawMessage { return c.extra }

type ChatContentFileURL struct {
	extra    map[string]json.RawMessage
	name     string
	mimeType string
	address  url.URL
}

func (c *ChatContentFileURL) _ChatContent() {}

func (c *ChatContentFileURL) Type() (mediaType string, params map[string]string) {
	//nolint:errcheck // handled via default values
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentFileURL) URL() *url.URL                     { return clone(&c.address) }
func (c *ChatContentFileURL) Extra() map[string]json.RawMessage { return c.extra }
func (c *ChatContentFileURL) Name() string                      { return c.name }

type ChatContentImageURL struct {
	extra    map[string]json.RawMessage
	mimeType string
	address  url.URL
	detail   ImageURLDetail
}

func (c *ChatContentImageURL) _ChatContent() {}

func (c *ChatContentImageURL) Type() (mediaType string, params map[string]string) {
	//nolint:errcheck // handled via default values
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentImageURL) URL() *url.URL                     { return clone(&c.address) }
func (c *ChatContentImageURL) Extra() map[string]json.RawMessage { return c.extra }
func (c *ChatContentImageURL) Detail() ImageURLDetail            { return c.detail }

type ImageURLDetail uint8

const (
	ImageURLDetailInvalid ImageURLDetail = iota
	ImageURLDetailHigh                   // high
	ImageURLDetailLow                    // low
	ImageURLDetailAuto                   // auto
)

func clone[T any](v *T) *T {
	t := new(T)
	*t = *v

	return t
}
