package domain

import (
	"time"
)

// DocumentCreate representa los datos necesarios para persistir un nuevo documento.
type DocumentCreate struct {
	Filename       string    `json:"filename" bson:"filename" validate:"required"`
	TextContent    string    `json:"text_content" bson:"text_content" validate:"required"`
	Checksum       string    `json:"checksum" bson:"checksum" validate:"required"`
	FileSizeBytes  int64     `json:"file_size_bytes" bson:"file_size_bytes" validate:"required,gt=0"`
	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
}

// DocumentResponse es la representación pública de un documento (lo que ve el cliente).
type DocumentResponse struct {
	ID             string    `json:"id" bson:"_id,omitempty"`
	Filename       string    `json:"filename" bson:"filename"`
	TextContent    string    `json:"text_content" bson:"text_content"`
	Checksum       string    `json:"checksum" bson:"checksum"`
	FileSizeBytes  int64     `json:"file_size_bytes" bson:"file_size_bytes"`
	CreatedAt      time.Time `json:"created_at" bson:"created_at"`
}

// DocumentUpdate representa los campos actualizables de un documento.
// Todos son opcionales: el cliente solo envía lo que quiere cambiar (PATCH semántico).
type DocumentUpdate struct {
	Filename    *string `json:"filename,omitempty" bson:"filename,omitempty"`
	TextContent *string `json:"text_content,omitempty" bson:"text_content,omitempty"`
}

// GetChanges devuelve solo los campos provistos por el cliente.
func (u *DocumentUpdate) GetChanges() map[string]any {
	changes := make(map[string]any)
	if u.Filename != nil {
		changes["filename"] = *u.Filename
	}
	if u.TextContent != nil {
		changes["text_content"] = *u.TextContent
	}
	return changes
}

// PageQuery encapsula los parámetros de paginación de una consulta de listado.
type PageQuery struct {
	Skip  int64 `form:"skip" binding:"gte=0"`
	Limit int64 `form:"limit" binding:"gte=1,lte=500"`
}