package unified

import "encoding/json"

type ContentKind string

const (
	ContentKindText      ContentKind = "text"
	ContentKindImage     ContentKind = "image"
	ContentKindAudio     ContentKind = "audio"
	ContentKindVideo     ContentKind = "video"
	ContentKindFile      ContentKind = "file"
	ContentKindReasoning ContentKind = "reasoning"
	ContentKindRefusal   ContentKind = "refusal"
	ContentKindToolCall  ContentKind = "tool_call"
)

type ContentPart interface {
	contentKind() ContentKind
}

type TextPart struct {
	Text string `json:"text,omitempty"`
}

func (TextPart) contentKind() ContentKind { return ContentKindText }

type ImagePart struct {
	Source BlobSource `json:"source"`
	Alt    string     `json:"alt,omitempty"`
}

func (ImagePart) contentKind() ContentKind { return ContentKindImage }

type AudioPart struct {
	Source BlobSource `json:"source"`
}

func (AudioPart) contentKind() ContentKind { return ContentKindAudio }

type VideoPart struct {
	Source BlobSource `json:"source"`
}

func (VideoPart) contentKind() ContentKind { return ContentKindVideo }

type FilePart struct {
	Source   BlobSource `json:"source"`
	Filename string     `json:"filename,omitempty"`
	MIMEType string     `json:"mime_type,omitempty"`
}

func (FilePart) contentKind() ContentKind { return ContentKindFile }

type ReasoningPart struct {
	Text string `json:"text,omitempty"`
}

func (ReasoningPart) contentKind() ContentKind { return ContentKindReasoning }

type RefusalPart struct {
	Text string `json:"text,omitempty"`
}

func (RefusalPart) contentKind() ContentKind { return ContentKindRefusal }

type BlobSourceKind string

const (
	BlobSourceURL    BlobSourceKind = "url"
	BlobSourceBase64 BlobSourceKind = "base64"
	BlobSourceBytes  BlobSourceKind = "bytes"
	BlobSourceFileID BlobSourceKind = "file_id"
)

type BlobSource struct {
	Kind     BlobSourceKind  `json:"kind"`
	URL      string          `json:"url,omitempty"`
	Base64   string          `json:"base64,omitempty"`
	Bytes    []byte          `json:"bytes,omitempty"`
	FileID   string          `json:"file_id,omitempty"`
	MIMEType string          `json:"mime_type,omitempty"`
	Meta     map[string]any  `json:"meta,omitempty"`
	Raw      json.RawMessage `json:"raw,omitempty"`
}
