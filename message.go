package agent

import (
	"context"
	"encoding/base64"
	"fmt"

	"gopkg.in/yaml.v2"
)

type ContentFn func(context.Context) (string, error)

type Image struct {
	Name string
	Data []byte
}

type Message struct {
	Role    Role
	content string

	imageData []Image

	// TODO: add name concept which is part of openai api anyway. Might be useful, but also better to not
	// overuse FunctionCallName for functions.
	Name string

	FunctionCallName string
	FunctionCallArgs string

	contentFn ContentFn
	attrs     map[string]string
}

func (m *Message) Content(ctx context.Context) (string, error) {
	if m.contentFn != nil {
		return m.contentFn(ctx)
	}
	return m.content, nil
}

func (m *Message) Images() []Image {
	i := make([]Image, len(m.imageData))
	copy(i, m.imageData)
	return i
}

func (m *Message) AddImage(name string, data []byte) {
	m.imageData = append(m.imageData, Image{Name: name, Data: data})
}

func (m *Message) SetAttr(key, value string) {
	m.attrs[key] = value
}

func (m *Message) GetAttr(key string) string {
	return m.attrs[key]
}

func (m *Message) Tag(key string) {
	m.attrs[key] = ""
}

func (m *Message) ClearTag(key string) {
	delete(m.attrs, key)
}

func (m *Message) HasTag(key string) bool {
	_, ok := m.attrs[key]
	return ok
}

func newMessage() *Message {
	return &Message{attrs: make(map[string]string)}
}

func NewContentMessage(role Role, content string) *Message {
	m := newMessage()
	m.Role = role
	m.content = content

	return m
}

func NewImageMessage(role Role, content string, imageName string, imageData []byte) *Message {
	m := newMessage()
	m.Role = role
	m.content = content
	m.AddImage(imageName, imageData)
	return m
}

func NewDynamicMessage(role Role, contentFn ContentFn) *Message {
	m := newMessage()
	m.Role = role
	m.contentFn = contentFn
	return m
}

func NewMessageFromMessage(m *Message) *Message {
	nm := newMessage()
	nm.Role = m.Role
	nm.content = m.content
	nm.Name = m.Name
	nm.FunctionCallName = m.FunctionCallName
	nm.FunctionCallArgs = m.FunctionCallArgs
	nm.contentFn = m.contentFn
	nm.imageData = make([]Image, len(m.imageData))
	copy(nm.imageData, m.imageData)

	for k, v := range m.attrs {
		nm.attrs[k] = v
	}
	return nm
}

func ExportMessagesToYAML(ctx context.Context, messages []*Message) (string, error) {
	yamlMessages := make([]map[string]interface{}, len(messages))

	for i, m := range messages {
		content, err := m.Content(ctx)
		if err != nil {
			return "", fmt.Errorf("error getting message content: %w", err)
		}
		yamlMessage := make(map[string]interface{})
		yamlMessage["Role"] = m.Role
		yamlMessage["Content"] = content

		if len(m.imageData) > 0 {
			yamlMessage["Images"] = make([]map[string]string, 0, len(m.imageData))
			images := make([]interface{}, 0, len(m.imageData))
			for _, img := range m.imageData {
				dst := make([]byte, base64.StdEncoding.EncodedLen(len(img.Data)))
				base64.StdEncoding.Encode(dst, img.Data)
				img := map[string]string{
					"name": img.Name,
					"data": string(dst),
				}
				images = append(images, img)
			}
			yamlMessage["Images"] = images
		}

		// TODO: Functions
		// TODO: attrs

		yamlMessages[i] = yamlMessage
	}

	bytes, err := yaml.Marshal(yamlMessages)
	if err != nil {
		return "", fmt.Errorf("error marshaling messages to YAML: %w", err)
	}

	return string(bytes), nil
}

func ImportMessagesFromYAML(yamlString string) ([]*Message, error) {
	var yamlMessages []map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlString), &yamlMessages); err != nil {
		return nil, fmt.Errorf("error unmarshaling YAML: %w", err)
	}

	var messages []*Message
	for _, ym := range yamlMessages {
		messages = append(messages, &Message{
			Role:    Role(ym["Role"].(string)),
			content: ym["Content"].(string),
		})
	}

	return messages, nil
}
