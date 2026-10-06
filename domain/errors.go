package domain

import "errors"

// DomainError es la base de todos los errores de dominio.
// code es un identificador estable y legible del problema (usado por la capa HTTP
// para resolver el status HTTP sin acoplar dominio y HTTP).
type DomainError struct {
	Code   string
	Detail string
}

func (e *DomainError) Error() string {
	return e.Detail
}

// Error codes estables para mapeo a RFC 9457
const (
	ErrorCodeDocumentNotFound      = "DOCUMENT_NOT_FOUND"
	ErrorCodeInvalidDocumentID     = "INVALID_DOCUMENT_ID"
	ErrorCodeDocumentAlreadyExists = "DOCUMENT_ALREADY_EXISTS"
	ErrorCodeInternal              = "INTERNAL_ERROR"
)

// DocumentNotFoundError indica que no se encontró un documento.
var ErrDocumentNotFound = errors.New("documento no encontrado")

type DocumentNotFoundError struct {
	DomainError
}

func NewDocumentNotFoundError(docID string) *DocumentNotFoundError {
	return &DocumentNotFoundError{
		DomainError: DomainError{
			Code:   ErrorCodeDocumentNotFound,
			Detail: "Documento con id '" + docID + "' no encontrado.",
		},
	}
}

func (e *DocumentNotFoundError) Is(target error) bool {
	return target == ErrDocumentNotFound
}

// InvalidDocumentIdError indica que el ID no tiene formato válido.
var ErrInvalidDocumentID = errors.New("ID de documento inválido")

type InvalidDocumentIdError struct {
	DomainError
}

func NewInvalidDocumentIdError(docID string) *InvalidDocumentIdError {
	return &InvalidDocumentIdError{
		DomainError: DomainError{
			Code:   ErrorCodeInvalidDocumentID,
			Detail: "'" + docID + "' no es un ID válido.",
		},
	}
}

func (e *InvalidDocumentIdError) Is(target error) bool {
	return target == ErrInvalidDocumentID
}

// DocumentAlreadyExistsError indica que el documento ya existe (duplicado por checksum).
var ErrDocumentAlreadyExists = errors.New("documento ya existe")

type DocumentAlreadyExistsError struct {
	DomainError
}

func NewDocumentAlreadyExistsError(checksum string) *DocumentAlreadyExistsError {
	return &DocumentAlreadyExistsError{
		DomainError: DomainError{
			Code:   ErrorCodeDocumentAlreadyExists,
			Detail: "El documento ya fue cargado previamente (checksum: " + checksum + ").",
		},
	}
}

func (e *DocumentAlreadyExistsError) Is(target error) bool {
	return target == ErrDocumentAlreadyExists
}

// InternalError para errores inesperados.
var ErrInternal = errors.New("error interno")

type InternalError struct {
	DomainError
}

func NewInternalError(detail string) *InternalError {
	return &InternalError{
		DomainError: DomainError{
			Code:   ErrorCodeInternal,
			Detail: detail,
		},
	}
}

func (e *InternalError) Is(target error) bool {
	return target == ErrInternal
}

// SlugFor mapea un error de dominio a su slug RFC 9457 estable.
func SlugFor(err error) string {
	switch {
	case errors.Is(err, ErrDocumentNotFound):
		return "document-not-found"
	case errors.Is(err, ErrInvalidDocumentID):
		return "invalid-document-id"
	case errors.Is(err, ErrDocumentAlreadyExists):
		return "document-already-exists"
	default:
		return "server-error"
	}
}