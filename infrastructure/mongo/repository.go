package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"microservicio-db-go/domain"
)

var _ domain.DocumentRepository = (*Repository)(nil)
var _ domain.HealthRepository = (*Repository)(nil)

const (
	indexChecksum = "checksum_unique"
)

// Repository persiste documentos en MongoDB. Posee un único cliente
// (connection pooling manejado por el driver) y expone Ping para health
// y Disconnect para apagado grácil.
type Repository struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// NewRepository conecta una vez, falla rápido si la instancia no es accesible,
// y asegura que exista el índice único de checksum. El llamador debe llamar
// Disconnect en el apagado.
func NewRepository(ctx context.Context, uri, db, collection string) (*Repository, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetConnectTimeout(5 * time.Second).SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("conectar mongodb: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	repo := &Repository{
		client:     client,
		collection: client.Database(db).Collection(collection),
	}
	if err := repo.ensureIndex(ctx); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("asegurar índice: %w", err)
	}
	return repo, nil
}

func (r *Repository) ExistsByChecksum(ctx context.Context, checksum string) (bool, error) {
	doc := r.collection.FindOne(ctx, bson.M{"checksum": checksum}, options.FindOne().SetProjection(bson.M{"_id": 1}))
	if doc.Err() != nil {
		if doc.Err() == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, fmt.Errorf("buscar por checksum: %w", doc.Err())
	}
	return true, nil
}

func (r *Repository) Create(ctx context.Context, data *domain.DocumentCreate) (*domain.DocumentResponse, error) {
	payload := bson.M{
		"filename":        data.Filename,
		"text_content":    data.TextContent,
		"checksum":        data.Checksum,
		"file_size_bytes": data.FileSizeBytes,
		"created_at":      data.CreatedAt,
	}
	result, err := r.collection.InsertOne(ctx, payload)
	if err != nil {
		// Verificar si es error de clave duplicada (código 11000)
		if mongo.IsDuplicateKeyError(err) {
			return nil, domain.NewDocumentAlreadyExistsError(data.Checksum)
		}
		return nil, fmt.Errorf("insertar documento: %w", err)
	}
	return &domain.DocumentResponse{
		ID:             result.InsertedID.(bson.ObjectID).Hex(),
		Filename:       data.Filename,
		TextContent:    data.TextContent,
		Checksum:       data.Checksum,
		FileSizeBytes:  data.FileSizeBytes,
		CreatedAt:      data.CreatedAt,
	}, nil
}

func (r *Repository) GetAll(ctx context.Context, query domain.PageQuery) ([]*domain.DocumentResponse, error) {
	opts := options.Find().
		SetSkip(query.Skip).
		SetLimit(query.Limit)
	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("listar documentos: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []*domain.DocumentResponse
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decodificar documento: %w", err)
		}
		docs = append(docs, mapToResponse(doc))
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterar cursor: %w", err)
	}
	return docs, nil
}

func (r *Repository) GetByID(ctx context.Context, docID string) (*domain.DocumentResponse, error) {
	objID, err := bson.ObjectIDFromHex(docID)
	if err != nil {
		return nil, domain.NewInvalidDocumentIdError(docID)
	}
	var doc bson.M
	err = r.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, domain.NewDocumentNotFoundError(docID)
		}
		return nil, fmt.Errorf("buscar por ID: %w", err)
	}
	return mapToResponse(doc), nil
}

func (r *Repository) Update(ctx context.Context, docID string, changes map[string]any) (*domain.DocumentResponse, error) {
	objID, err := bson.ObjectIDFromHex(docID)
	if err != nil {
		return nil, domain.NewInvalidDocumentIdError(docID)
	}
	var updatedDoc bson.M
	err = r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": changes},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updatedDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, domain.NewDocumentNotFoundError(docID)
		}
		return nil, fmt.Errorf("actualizar documento: %w", err)
	}
	return mapToResponse(updatedDoc), nil
}

func (r *Repository) Delete(ctx context.Context, docID string) error {
	objID, err := bson.ObjectIDFromHex(docID)
	if err != nil {
		return domain.NewInvalidDocumentIdError(docID)
	}
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return fmt.Errorf("eliminar documento: %w", err)
	}
	if result.DeletedCount == 0 {
		return domain.NewDocumentNotFoundError(docID)
	}
	return nil
}

func (r *Repository) EnsureIndexes(ctx context.Context) error {
	return r.ensureIndex(ctx)
}

func (r *Repository) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("ping mongodb: %w", err)
	}
	return nil
}

func (r *Repository) Disconnect(ctx context.Context) error {
	if err := r.client.Disconnect(ctx); err != nil {
		return fmt.Errorf("desconectar mongodb: %w", err)
	}
	return nil
}

// ensureIndex crea el índice único sobre checksum; crear un índice idéntico es idempotente.
func (r *Repository) ensureIndex(ctx context.Context) error {
	cursor, err := r.collection.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("listar índices: %w", err)
	}
	defer cursor.Close(ctx)

	var indexes []struct {
		Keys   bson.D `bson:"key"`
		Unique bool   `bson:"unique"`
	}
	if err := cursor.All(ctx, &indexes); err != nil {
		return fmt.Errorf("decodificar índices: %w", err)
	}
	for _, index := range indexes {
		if len(index.Keys) != 1 || index.Keys[0].Key != "checksum" {
			continue
		}
		if index.Keys[0].Value != int32(1) && index.Keys[0].Value != int64(1) && index.Keys[0].Value != int(1) {
			continue
		}
		if index.Unique {
			return nil
		}
		return fmt.Errorf("el índice existente sobre checksum no es único")
	}

	_, err = r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "checksum", Value: 1}},
		Options: options.Index().SetName(indexChecksum).SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("crear índice %s: %w", indexChecksum, err)
	}
	return nil
}

func mapToResponse(doc bson.M) *domain.DocumentResponse {
	var id string
	if oid, ok := doc["_id"].(bson.ObjectID); ok {
		id = oid.Hex()
	}
	return &domain.DocumentResponse{
		ID:             id,
		Filename:       getString(doc, "filename"),
		TextContent:    getString(doc, "text_content"),
		Checksum:       getString(doc, "checksum"),
		FileSizeBytes:  getInt64(doc, "file_size_bytes"),
		CreatedAt:      getTime(doc, "created_at"),
	}
}

func getString(doc bson.M, key string) string {
	if v, ok := doc[key].(string); ok {
		return v
	}
	return ""
}

func getInt64(doc bson.M, key string) int64 {
	switch v := doc[key].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	}
	return 0
}

func getTime(doc bson.M, key string) time.Time {
	if v, ok := doc[key].(time.Time); ok {
		return v
	}
	if v, ok := doc[key].(bson.DateTime); ok {
		return v.Time()
	}
	return time.Time{}
}