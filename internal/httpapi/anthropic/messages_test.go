package anthropic

import (
	"reflect"
	"testing"

	"github.com/crmmc/copilotpi/internal/flow"
)

func TestToFlowRequest_Basic(t *testing.T) {
	req := &MessageRequest{
		Model: "claude-3-5-sonnet-20241022",
		System: "You are a helpful assistant",
		Messages: []flow.Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	flowReq := toFlowRequest(req)
	
	if flowReq.Model != req.Model {
		t.Errorf("expected model %s, got %s", req.Model, flowReq.Model)
	}
	
	if len(flowReq.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(flowReq.Messages))
	}
	
	if flowReq.Messages[0].Role != "system" {
		t.Errorf("expected first message role to be system, got %s", flowReq.Messages[0].Role)
	}

	if !reflect.DeepEqual(flowReq.Messages[0].Content, req.System) {
		t.Errorf("expected system content to match")
	}

	if flowReq.Messages[1].Role != "user" || flowReq.Messages[1].Content != "Hello!" {
		t.Errorf("expected second message to be user")
	}
}

func TestToFlowRequest_NoSystem(t *testing.T) {
	req := &MessageRequest{
		Model: "dummy",
		Messages: []flow.Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	flowReq := toFlowRequest(req)
	
	if len(flowReq.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(flowReq.Messages))
	}
	
	if flowReq.Messages[0].Role != "user" {
		t.Errorf("expected first message role to be user, got %s", flowReq.Messages[0].Role)
	}
}
