package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

// llmDumper writes LLM request/response pairs to numbered JSON files.
type llmDumper struct {
	dir     string
	counter atomic.Uint64
}

type llmDumpRequest struct {
	Timestamp string                     `json:"timestamp"`
	AgentID   string                     `json:"agent_id,omitempty"`
	Iteration int                        `json:"iteration"`
	Model     string                     `json:"model"`
	Messages  []providers.Message        `json:"messages"`
	Tools     []providers.ToolDefinition `json:"tools,omitempty"`
	Options   map[string]any             `json:"options,omitempty"`
}

type llmDumpResponse struct {
	Timestamp    string               `json:"timestamp"`
	Content      string               `json:"content,omitempty"`
	ToolCalls    []providers.ToolCall `json:"tool_calls,omitempty"`
	FinishReason string               `json:"finish_reason,omitempty"`
	Usage        *providers.UsageInfo `json:"usage,omitempty"`
	Error        string               `json:"error,omitempty"`
}

type llmDumpEntry struct {
	Seq      uint64           `json:"seq"`
	Request  *llmDumpRequest  `json:"request,omitempty"`
	Response *llmDumpResponse `json:"response,omitempty"`
}

func newLLMDumper(dir string) *llmDumper {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		logger.WarnCF("agent", "Failed to create LLM dump directory", map[string]any{
			"dir":   dir,
			"error": err.Error(),
		})
		return nil
	}
	logger.InfoCF("agent", "LLM dump enabled", map[string]any{"dir": dir})
	return &llmDumper{dir: dir}
}

func (d *llmDumper) dumpRequest(agentID string, iteration int, model string,
	messages []providers.Message, tools []providers.ToolDefinition,
	opts map[string]any) uint64 {
	if d == nil {
		return 0
	}
	seq := d.counter.Add(1)
	entry := llmDumpEntry{
		Seq: seq,
		Request: &llmDumpRequest{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			AgentID:   agentID,
			Iteration: iteration,
			Model:     model,
			Messages:  messages,
			Tools:     tools,
			Options:   opts,
		},
	}
	d.write(seq, "req", entry)
	return seq
}

func (d *llmDumper) dumpResponse(seq uint64, resp *providers.LLMResponse, err error) {
	if d == nil {
		return
	}
	dr := &llmDumpResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if resp != nil {
		dr.Content = resp.Content
		dr.ToolCalls = resp.ToolCalls
		dr.FinishReason = resp.FinishReason
		dr.Usage = resp.Usage
	}
	if err != nil {
		dr.Error = err.Error()
	}
	entry := llmDumpEntry{
		Seq:      seq,
		Response: dr,
	}
	d.write(seq, "resp", entry)
}

func (d *llmDumper) write(seq uint64, suffix string, v any) {
	filename := filepath.Join(d.dir, fmt.Sprintf("%d_%s.json", seq, suffix))
	data, jsonErr := json.MarshalIndent(v, "", "  ")
	if jsonErr != nil {
		logger.WarnCF("agent", "Failed to marshal LLM dump", map[string]any{
			"file":  filename,
			"error": jsonErr.Error(),
		})
		return
	}
	if writeErr := os.WriteFile(filename, data, 0o640); writeErr != nil {
		logger.WarnCF("agent", "Failed to write LLM dump", map[string]any{
			"file":  filename,
			"error": writeErr.Error(),
		})
	}
}
