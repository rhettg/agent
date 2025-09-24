package openairesponses

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/responses"
	"github.com/rhettg/agent"
)

// mapMessagesToInputItems converts Agent messages to OpenAI Responses API input items
func (p *provider) mapMessagesToInputItems(ctx context.Context, msgs []*agent.Message) (responses.ResponseNewParamsInputUnion, error) {
	var items []responses.ResponseInputItemUnionParam
	
	for _, m := range msgs {
		content, err := m.Content(ctx)
		if err != nil {
			return responses.ResponseNewParamsInputUnion{}, fmt.Errorf("failed to get message content: %w", err)
		}

		switch m.Role {
		case agent.RoleSystem:
			// For simple text messages, we can use the string version
			items = append(items, responses.ResponseInputItemParamOfMessage(content, "system"))

		case agent.RoleUser:
			// For simple text messages, we can use the string version
			if len(m.Images()) == 0 {
				items = append(items, responses.ResponseInputItemParamOfMessage(content, "user"))
			} else {
				// For messages with images, use content list
				contentParts := responses.ResponseInputMessageContentListParam{}
				if content != "" {
					contentParts = append(contentParts, responses.ResponseInputContentParamOfInputText(content))
				}
				for _, img := range m.Images() {
					mimeType := mimeType(img.Name)
					imageURL := encodeImageURL(mimeType, img.Data)
					contentParts = append(contentParts, responses.ResponseInputContentUnionParam{
						OfInputImage: &responses.ResponseInputImageParam{
							Detail:   "auto",
							ImageURL: openai.String(imageURL),
						},
					})
				}
				items = append(items, responses.ResponseInputItemParamOfMessage(contentParts, "user"))
			}

		case agent.RoleAssistant:
			// For now, treat all assistant messages as simple messages
			// TODO: Handle tool calls properly when we understand the correct API structure
			items = append(items, responses.ResponseInputItemParamOfMessage(content, "assistant"))
			
			// NOTE: We do NOT send reasoning back to the model as input.
			// Reasoning is response-only metadata for consumers and should not
			// be part of the conversation context sent to the model.

		case agent.RoleTool:
			// Tool response
			items = append(items, responses.ResponseInputItemParamOfFunctionCallOutput(
				m.ToolCallID,
				content,
			))
		}
	}
	
	return responses.ResponseNewParamsInputUnion{
		OfInputItemList: items,
	}, nil
}

// mapResponseToMessage converts OpenAI Responses API response to an Agent message
func (p *provider) mapResponseToMessage(resp *responses.Response) (*agent.Message, error) {
	// Get the text content from the response
	content := resp.OutputText()
	
	// Create the message
	msg := agent.NewContentMessage(agent.RoleAssistant, content)
	
	// TODO: Extract tool calls from response output if present
	// TODO: Extract reasoning from response if present
	
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
