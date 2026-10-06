package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDocumentUpdate_GetChanges(t *testing.T) {
	tests := []struct {
		name     string
		update   DocumentUpdate
		expected map[string]any
	}{
		{
			name: "solo filename",
			update: DocumentUpdate{
				Filename: stringPtr("nuevo.pdf"),
			},
			expected: map[string]any{"filename": "nuevo.pdf"},
		},
		{
			name: "solo text_content",
			update: DocumentUpdate{
				TextContent: stringPtr("nuevo texto"),
			},
			expected: map[string]any{"text_content": "nuevo texto"},
		},
		{
			name: "ambos campos",
			update: DocumentUpdate{
				Filename:    stringPtr("nuevo.pdf"),
				TextContent: stringPtr("nuevo texto"),
			},
			expected: map[string]any{
				"filename":     "nuevo.pdf",
				"text_content": "nuevo texto",
			},
		},
		{
			name:     "vacío retorna mapa vacío",
			update:   DocumentUpdate{},
			expected: map[string]any{},
		},
		{
			name: "valor falsy explícito (string vacío) se envía",
			update: DocumentUpdate{
				TextContent: stringPtr(""),
			},
			expected: map[string]any{"text_content": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.update.GetChanges()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPageQuery_Defaults(t *testing.T) {
	q := PageQuery{}
	assert.Equal(t, int64(0), q.Skip)
	assert.Equal(t, int64(100), q.Limit)
}

func stringPtr(s string) *string {
	return &s
}

func TestDocumentCreate_Validation(t *testing.T) {
	now := time.Now().UTC()
	doc := DocumentCreate{
		Filename:       "test.pdf",
		TextContent:    "contenido",
		Checksum:       "abc123",
		FileSizeBytes:  1024,
		CreatedAt:      now,
	}
	assert.Equal(t, "test.pdf", doc.Filename)
	assert.Equal(t, int64(1024), doc.FileSizeBytes)
	assert.Equal(t, now, doc.CreatedAt)
}