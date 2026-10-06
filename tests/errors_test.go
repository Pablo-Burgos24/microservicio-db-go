package tests

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedCode string
		expectedMsg string
	}{
		{
			name:         "DocumentNotFoundError",
			err:          NewDocumentNotFoundError("60d5ecb8b392d70008051234"),
			expectedCode: ErrorCodeDocumentNotFound,
			expectedMsg:  "Documento con id '60d5ecb8b392d70008051234' no encontrado.",
		},
		{
			name:         "InvalidDocumentIdError",
			err:          NewInvalidDocumentIdError("invalid-id"),
			expectedCode: ErrorCodeInvalidDocumentID,
			expectedMsg:  "'invalid-id' no es un ID válido.",
		},
		{
			name:         "DocumentAlreadyExistsError",
			err:          NewDocumentAlreadyExistsError("abc123checksum"),
			expectedCode: ErrorCodeDocumentAlreadyExists,
			expectedMsg:  "El documento ya fue cargado previamente (checksum: abc123checksum).",
		},
		{
			name:         "InternalError",
			err:          NewInternalError("algo falló"),
			expectedCode: ErrorCodeInternal,
			expectedMsg:  "algo falló",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var de *DomainError
			assert.ErrorAs(t, tt.err, &de)
			assert.Equal(t, tt.expectedCode, de.Code)
			assert.Equal(t, tt.expectedMsg, de.Detail)
			assert.Equal(t, tt.expectedMsg, tt.err.Error())
		})
	}
}

func TestErrorIs(t *testing.T) {
	notFound := NewDocumentNotFoundError("id1")
	invalidID := NewInvalidDocumentIdError("id2")
	alreadyExists := NewDocumentAlreadyExistsError("checksum")
	internal := NewInternalError("error")

	assert.True(t, errors.Is(notFound, ErrDocumentNotFound))
	assert.True(t, errors.Is(invalidID, ErrInvalidDocumentID))
	assert.True(t, errors.Is(alreadyExists, ErrDocumentAlreadyExists))
	assert.True(t, errors.Is(internal, ErrInternal))

	assert.False(t, errors.Is(notFound, ErrInvalidDocumentID))
	assert.False(t, errors.Is(invalidID, ErrDocumentAlreadyExists))
}

func TestSlugFor(t *testing.T) {
	assert.Equal(t, "document-not-found", SlugFor(NewDocumentNotFoundError("id")))
	assert.Equal(t, "invalid-document-id", SlugFor(NewInvalidDocumentIdError("id")))
	assert.Equal(t, "document-already-exists", SlugFor(NewDocumentAlreadyExistsError("checksum")))
	assert.Equal(t, "server-error", SlugFor(NewInternalError("error")))
	assert.Equal(t, "server-error", SlugFor(errors.New("error desconocido")))
}