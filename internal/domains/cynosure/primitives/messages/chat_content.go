package messages

import (
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

var _ ChatContent = (*ChatContentText)(nil)
var _ ChatContent = (*ChatContentAudioURL)(nil)
var _ ChatContent = (*ChatContentVideoURL)(nil)
var _ ChatContent = (*ChatContentFileURL)(nil)
var _ ChatContent = (*ChatContentImageURL)(nil)

type ChatContentText struct {
	text  string
	extra map[string]json.RawMessage
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

func (c *ChatContentText) URL() *url.URL                     { return &url.URL{Scheme: "text"} }
func (c *ChatContentText) Extra() map[string]json.RawMessage { return c.extra }

func (c *ChatContentText) Text() string { return c.text }

type ChatContentAudioURL struct {
	url      url.URL
	mimeType string
	extra    map[string]json.RawMessage
}

func (c *ChatContentAudioURL) _ChatContent() {}

func (c *ChatContentAudioURL) Type() (mediaType string, params map[string]string) {
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentAudioURL) URL() *url.URL                     { u := new(url.URL); *u = c.url; return u }
func (c *ChatContentAudioURL) Extra() map[string]json.RawMessage { return c.extra }

type ChatContentVideoURL struct {
	url      url.URL
	mimeType string
	extra    map[string]json.RawMessage
}

func (c *ChatContentVideoURL) _ChatContent() {}

func (c *ChatContentVideoURL) Type() (mediaType string, params map[string]string) {
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentVideoURL) URL() *url.URL                     { u := new(url.URL); *u = c.url; return u }
func (c *ChatContentVideoURL) Extra() map[string]json.RawMessage { return c.extra }

type ChatContentFileURL struct {
	name     string
	url      url.URL
	mimeType string
	extra    map[string]json.RawMessage
}

func (c *ChatContentFileURL) _ChatContent() {}

func (c *ChatContentFileURL) Type() (mediaType string, params map[string]string) {
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentFileURL) URL() *url.URL                     { u := new(url.URL); *u = c.url; return u }
func (c *ChatContentFileURL) Extra() map[string]json.RawMessage { return c.extra }

type ChatContentImageURL struct {
	url      url.URL
	detail   ImageURLDetail
	mimeType string
	extra    map[string]json.RawMessage
}

func (c *ChatContentImageURL) _ChatContent() {}

func (c *ChatContentImageURL) Type() (mediaType string, params map[string]string) {
	mediaType, params, _ = mime.ParseMediaType(c.mimeType)
	return mediaType, params
}

func (c *ChatContentImageURL) URL() *url.URL                     { u := new(url.URL); *u = c.url; return u }
func (c *ChatContentImageURL) Extra() map[string]json.RawMessage { return c.extra }

type ImageURLDetail uint8

const (
	ImageURLDetailInvalid ImageURLDetail = iota
	ImageURLDetailHigh                   // high
	ImageURLDetailLow                    // low
	ImageURLDetailAuto                   // auto
)
