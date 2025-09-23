package openairesponses

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rhettg/agent"
)

// mapMessagesToInputItems converts Agent messages to OpenAI Responses API input items
func (p *provider) mapMessagesToInputItems(ctx context.Context, msgs []*agent.Message) ([]interface{}, error) {
	var items []interface{}
	
	for _, m := range msgs {
		content, err := m.Content(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get message content: %w", err)
		}

		switch m.Role {
		case agent.RoleSystem:
			items = append(items, map[string]interface{}{
				"type": "message",
				"role": "system",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": content,
					},
				},
			})

		case agent.RoleUser:
			contentParts := []map[string]interface{}{}
			
			// Add text content if present
			if content != "" {
				contentParts = append(contentParts, map[string]interface{}{
					"type": "text",
					"text": content,
				})
			}
			
			// Add images if present
			for _, img := range m.Images() {
				mimeType := mimeType(img.Name)
				imageURL := encodeImageURL(mimeType, img.Data)
				contentParts = append(contentParts, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": imageURL,
					},
				})
			}
			
			items = append(items, map[string]interface{}{
				"type": "message",
				"role": "user",
				"content": contentParts,
			})

		case agent.RoleAssistant:
			// Assistant message with potential tool calls and reasoning
			contentParts := []map[string]interface{}{}
			
			if content != "" {
				contentParts = append(contentParts, map[string]interface{}{
					"type": "text",
					"text": content,
				})
			}
			
			// Add tool calls
			for _, tc := range m.ToolCalls {
				contentParts = append(contentParts, map[string]interface{}{
					"type": "function_call",
					"id":   tc.ID,
					"name": tc.Name,
					"arguments": tc.Arguments,
				})
			}
			
			msgItem := map[string]interface{}{
				"type": "message",
				"role": "assistant",
				"content": contentParts,
			}
			
			items = append(items, msgItem)
			
			// Add reasoning if present
			if m.Reasoning != nil {
				if m.Reasoning.Content != "" {
					items = append(items, map[string]interface{}{
						"type": "reasoning",
						"content": m.Reasoning.Content,
					})
				}
				if m.Reasoning.EncryptedContent != "" {
					items = append(items, map[string]interface{}{
						"type": "reasoning",
						"encrypted_content": m.Reasoning.EncryptedContent,
					})
				}
			}

		case agent.RoleTool:
			// Tool response
			items = append(items, map[string]interface{}{
				"type": "function_call_output",
				"call_id": m.ToolCallID,
				"output": content,
			})
		}
	}
	
	return items, nil
}

// mapOutputItemsToMessage converts OpenAI Responses API output items to an Agent message
func (p *provider) mapOutputItemsToMessage(items []interface{}) (*agent.Message, error) {
	msg := agent.NewContentMessage(agent.RoleAssistant, "")
	var contentParts []string
	var reasoning *agent.Reasoning
	
	for _, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		
		itemType, ok := itemMap["type"].(string)
		if !ok {
			continue
		}
		
		switch itemType {
		case "text":
			if text, ok := itemMap["text"].(string); ok {
				contentParts = append(contentParts, text)
			}
			
		case "function_call":
			// Tool call
			if id, ok := itemMap["id"].(string); ok {
				if name, ok := itemMap["name"].(string); ok {
					if args, ok := itemMap["arguments"].(string); ok {
						msg.ToolCalls = append(msg.ToolCalls, agent.ToolCall{
							ID:        id,
							Name:      name,
							Arguments: args,
						})
					}
				}
			}
			
		case "reasoning":
			if reasoning == nil {
				reasoning = &agent.Reasoning{}
			}
			
			if content, ok := itemMap["content"].(string); ok {
				reasoning.Content = content
			}
			if encryptedContent, ok := itemMap["encrypted_content"].(string); ok {
				reasoning.EncryptedContent = encryptedContent
			}
			if summaries, ok := itemMap["summaries"].([]interface{}); ok {
				for _, summary := range summaries {
					if s, ok := summary.(string); ok {
						reasoning.Summaries = append(reasoning.Summaries, s)
					}
				}
			}
		}
	}
	
	// Set the combined content
	if len(contentParts) > 0 {
		// Update the message content using reflection since content is private
		// We'll need to create a new message with the content
		newMsg := agent.NewContentMessage(agent.RoleAssistant, strings.Join(contentParts, ""))
		newMsg.ToolCalls = msg.ToolCalls
		newMsg.Reasoning = reasoning
		return newMsg, nil
	}
	
	msg.Reasoning = reasoning
	return msg, nil
}

func mimeType(name string) string {
	dot := strings.LastIndex(name, ".")
	if dot == -1 || dot == len(name)-1 {
		return "image/jpeg"
	}
	return "image/" + strings.ToLower(name[dot+1:])
}

func encodeImageURL(mimeType string, data []byte) string {
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(dst, data)

	var imageURL strings.Builder
	imageURL.WriteString("data:")
	imageURL.WriteString(mimeType)
	imageURL.WriteString(";base64,")
	imageURL.Write(dst)

	return imageURL.String()
}
