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

	// TODO: add name concept which is part of openai api anyway. Might be useful.
	Name string

	// Tool calling support
	ToolCalls []ToolCall  // Only for assistant messages
	ToolCallID string     // Only for tool response messages

	// Reasoning support (optional, primarily for assistant messages)
	ReasoningContent          string   `json:"reasoning_content,omitempty"`           // plain reasoning text
	ReasoningEncryptedContent string   `json:"reasoning_encrypted_content,omitempty"` // encrypted reasoning blob
	ReasoningSummaries        []string `json:"reasoning_summaries,omitempty"`         // human-readable summaries

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
	nm.ToolCalls = make([]ToolCall, len(m.ToolCalls))
	copy(nm.ToolCalls, m.ToolCalls)
	nm.ToolCallID = m.ToolCallID
	nm.contentFn = m.contentFn
	nm.imageData = make([]Image, len(m.imageData))
	copy(nm.imageData, m.imageData)

	// Copy reasoning fields
	nm.ReasoningContent = m.ReasoningContent
	nm.ReasoningEncryptedContent = m.ReasoningEncryptedContent
	if len(m.ReasoningSummaries) > 0 {
		nm.ReasoningSummaries = make([]string, len(m.ReasoningSummaries))
		copy(nm.ReasoningSummaries, m.ReasoningSummaries)
	}

	for k, v := range m.attrs {
		nm.attrs[k] = v
	}
	return nm
}

// HasToolCalls returns true if the message has tool calls
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// GetFirstToolCall returns the first tool call
func (m *Message) GetFirstToolCall() *ToolCall {
	if len(m.ToolCalls) > 0 {
		return &m.ToolCalls[0]
	}
	return nil
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

		// Add reasoning if present
		if m.ReasoningContent != "" || m.ReasoningEncryptedContent != "" || len(m.ReasoningSummaries) > 0 {
			reasoning := make(map[string]interface{})
			if m.ReasoningContent != "" {
				reasoning["content"] = m.ReasoningContent
			}
			if m.ReasoningEncryptedContent != "" {
				reasoning["encrypted_content"] = m.ReasoningEncryptedContent
			}
			if len(m.ReasoningSummaries) > 0 {
				reasoning["summaries"] = m.ReasoningSummaries
			}
			yamlMessage["Reasoning"] = reasoning
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
		msg := newMessage()
		msg.Role = Role(ym["Role"].(string))
		msg.content = ym["Content"].(string)
		
		// Import reasoning if present - try both possible map types for robustness
		if reasoningRaw, exists := ym["Reasoning"]; exists {
			var reasoningData map[string]interface{}
			
			// Try string-keyed map first, then interface-keyed map
			if stringMap, ok := reasoningRaw.(map[string]interface{}); ok {
				reasoningData = stringMap
			} else if interfaceMap, ok := reasoningRaw.(map[interface{}]interface{}); ok {
				// Convert interface{} keys to strings
				reasoningData = make(map[string]interface{})
				for k, v := range interfaceMap {
					if keyStr, ok := k.(string); ok {
						reasoningData[keyStr] = v
					}
				}
			}
			
			if reasoningData != nil {
				if content, ok := reasoningData["content"].(string); ok {
					msg.ReasoningContent = content
				}
				if encryptedContent, ok := reasoningData["encrypted_content"].(string); ok {
					msg.ReasoningEncryptedContent = encryptedContent
				}
				if summariesData, ok := reasoningData["summaries"].([]interface{}); ok {
					for _, summary := range summariesData {
						if s, ok := summary.(string); ok {
							msg.ReasoningSummaries = append(msg.ReasoningSummaries, s)
						}
					}
				}
			}
		}
		
		messages = append(messages, msg)
	}

	return messages, nil
}
