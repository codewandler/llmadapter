package messages

import "github.com/codewandler/llmadapter/anthropicwire"

type MessageRequest = anthropicwire.MessageRequest
type ThinkingConfig = anthropicwire.ThinkingConfig
type OutputConfig = anthropicwire.OutputConfig
type InputMessage = anthropicwire.InputMessage
type ContentBlock = anthropicwire.ContentBlock
type BlockSource = anthropicwire.BlockSource
type CacheControl = anthropicwire.CacheControl
type SystemContent = anthropicwire.SystemContent
type ToolDefinition = anthropicwire.ToolDefinition
type ToolChoiceWire = anthropicwire.ToolChoiceWire
type Event = anthropicwire.Event
type QuotaUsageEvent = anthropicwire.QuotaUsageEvent
type QuotaWindowUsage = anthropicwire.QuotaWindowUsage
type MessageStartEvent = anthropicwire.MessageStartEvent
type MessageResponse = anthropicwire.MessageResponse
type ContentBlockStartEvent = anthropicwire.ContentBlockStartEvent
type ContentBlockDeltaEvent = anthropicwire.ContentBlockDeltaEvent
type Delta = anthropicwire.Delta
type ContentBlockStopEvent = anthropicwire.ContentBlockStopEvent
type MessageDeltaEvent = anthropicwire.MessageDeltaEvent
type MessageDeltaBody = anthropicwire.MessageDeltaBody
type MessageStopEvent = anthropicwire.MessageStopEvent
type PingEvent = anthropicwire.PingEvent
type ErrorEventWire = anthropicwire.ErrorEventWire
type APIErrorBody = anthropicwire.APIErrorBody
type UsageWire = anthropicwire.UsageWire
type RawEventWire = anthropicwire.RawEventWire

var NewSystemContent = anthropicwire.NewSystemContent
