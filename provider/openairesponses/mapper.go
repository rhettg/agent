package openairesponses

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/responses"
	"github.com/rhettg/agent"
	"github.com/rhettg/agent/internal/imageutil"
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
					mimeType := imageutil.MimeType(img.Name)
					imageURL := imageutil.EncodeImageURL(mimeType, img.Data)
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
			// Add the assistant message
			items = append(items, responses.ResponseInputItemParamOfMessage(content, "assistant"))
			
			// Add tool calls if present
			for _, toolCall := range m.ToolCalls {
				items = append(items, responses.ResponseInputItemParamOfFunctionCall(
					toolCall.Arguments,
					toolCall.ID,
					toolCall.Name,
				))
			}
			
			// Add reasoning if present (encrypted reasoning is preserved for context)
			if m.ReasoningEncryptedContent != "" || len(m.ReasoningSummaries) > 0 {
				// Create reasoning summaries for input
				var summaries []responses.ResponseReasoningItemSummaryParam
				for _, summary := range m.ReasoningSummaries {
					summaries = append(summaries, responses.ResponseReasoningItemSummaryParam{
						Text: summary,
					})
				}
				
				// Use a generated ID for the reasoning item
				reasoningID := fmt.Sprintf("reasoning_%d", len(items))
				items = append(items, responses.ResponseInputItemParamOfReasoning(reasoningID, summaries))
			}

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
	
	// Extract reasoning and tool calls from response output items
	for _, outputItem := range resp.Output {
		// Extract reasoning
		if reasoningItem := outputItem.AsReasoning(); reasoningItem.Type != "" {
			// Extract reasoning content (text)
			var contentParts []string
			for _, contentItem := range reasoningItem.Content {
				contentParts = append(contentParts, contentItem.Text)
			}
			if len(contentParts) > 0 {
				msg.ReasoningContent = strings.Join(contentParts, "\n")
			}
			
			// Extract encrypted content if present
			if reasoningItem.EncryptedContent != "" {
				msg.ReasoningEncryptedContent = reasoningItem.EncryptedContent
			}
			
			// Extract reasoning summaries
			for _, summaryItem := range reasoningItem.Summary {
				msg.ReasoningSummaries = append(msg.ReasoningSummaries, summaryItem.Text)
			}
		}
		
		// Extract tool calls
		if functionCall := outputItem.AsFunctionCall(); functionCall.Type != "" {
			msg.ToolCalls = append(msg.ToolCalls, agent.ToolCall{
				ID:        functionCall.CallID,
				Name:      functionCall.Name,
				Arguments: functionCall.Arguments,
			})
		}
	}
	
	return msg, nil
}
