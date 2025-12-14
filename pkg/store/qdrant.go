package store

import (
	"ai-memory/pkg/config"
	"ai-memory/pkg/types"
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

type QdrantStore struct {
	client     *qdrant.Client
	collection string
}

func NewQdrantStore(cfg *config.Config) (*QdrantStore, error) {
	// Parse Host and Port from config
	host := cfg.QdrantAddr
	port := 6334 // Default

	if h, p, err := net.SplitHostPort(cfg.QdrantAddr); err == nil {
		host = h
		if pInt, err := strconv.Atoi(p); err == nil {
			port = pInt
		}
	}

	// Initialize client
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}

	return &QdrantStore{
		client:     client,
		collection: cfg.QdrantCollection,
	}, nil
}

// Init ensures the collection exists.
func (s *QdrantStore) Init(ctx context.Context, vectorSize int) error {
	// check existence
	exists, err := s.client.CollectionExists(ctx, s.collection)
	if err != nil {
		return err
	}
	if !exists {
		// Create collection
		err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: s.collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(vectorSize),
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
	}
	return nil
}

func (s *QdrantStore) Add(ctx context.Context, records []types.Record) error {
	var points []*qdrant.PointStruct
	for _, r := range records {
		if r.Embedding == nil {
			continue
		}

		// Convert []float32 to []float32 (already same)
		// Payload map
		payload := map[string]interface{}{
			"content":   r.Content,
			"timestamp": r.Timestamp.Format(time.RFC3339),
			"type":      string(r.Type),
			"metadata":  r.Metadata,
			// Flatten metadata into top level if needed, or keep as map? Qdrant handles JSON payload.
		}

		points = append(points, &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(r.ID),
			Vectors: qdrant.NewVectors(r.Embedding...),
			Payload: qdrant.NewValueMap(payload),
		})
	}

	operationInfo, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
		Wait:           func(b bool) *bool { return &b }(true),
	})
	if err != nil {
		return err
	}
	if operationInfo.Status != qdrant.UpdateStatus_Completed && operationInfo.Status != qdrant.UpdateStatus_Acknowledged {
		return fmt.Errorf("upsert not completed: %v", operationInfo.Status)
	}
	return nil
}

func (s *QdrantStore) Search(ctx context.Context, vector []float32, limit int, scoreThreshold float32, filters map[string]interface{}) ([]types.Record, error) {
	// Build Filter
	var qdrantFilter *qdrant.Filter
	if len(filters) > 0 {
		var conditions []*qdrant.Condition
		for k, v := range filters {
			// Create a match condition for each filter
			// Assuming exact match for string/int values
			valStr := fmt.Sprintf("%v", v)

			// Hack fix for user_id nesting in Qdrant Payload vs InMemory Metadata
			key := k
			if k == "user_id" {
				key = "metadata.user_id"
			}

			conditions = append(conditions, &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: key,
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Keyword{
								Keyword: valStr,
							},
						},
					},
				},
			})
		}
		qdrantFilter = &qdrant.Filter{
			Must: conditions,
		}
	}

	searchResult, err := s.client.GetPointsClient().Search(ctx, &qdrant.SearchPoints{
		CollectionName: s.collection,
		Vector:         vector,
		Limit:          uint64(limit),
		ScoreThreshold: &scoreThreshold,
		WithPayload:    qdrant.NewWithPayload(true),
		Filter:         qdrantFilter,
	})
	if err != nil {
		return nil, err
	}

	var records []types.Record
	for _, hit := range searchResult.GetResult() {
		payload := hit.Payload

		content := ""
		if val, ok := payload["content"]; ok {
			content = val.GetStringValue()
		}

		var ts time.Time
		if val, ok := payload["timestamp"]; ok {
			ts, _ = time.Parse(time.RFC3339, val.GetStringValue())
		}

		typeStr := ""
		if val, ok := payload["type"]; ok {
			typeStr = val.GetStringValue()
		}

		rec := types.Record{
			ID:        hit.Id.GetUuid(),
			Content:   content,
			Type:      types.MemoryType(typeStr),
			Timestamp: ts,
		}
		records = append(records, rec)
	}
	return records, nil
}

// Delete removes records by ID
func (s *QdrantStore) Delete(ctx context.Context, ids []string) error {
	var points []*qdrant.PointId
	for _, id := range ids {
		points = append(points, qdrant.NewIDUUID(id))
	}

	// Correct PointsSelector construction
	pointsSelector := &qdrant.PointsSelector{
		PointsSelectorOneOf: &qdrant.PointsSelector_Points{
			Points: &qdrant.PointsIdsList{Ids: points},
		},
	}

	_, err := s.client.GetPointsClient().Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points:         pointsSelector,
	})
	return err
}

// List is not efficiently supported by Vector DBs usually (scan),
// but needed for Summarize/Clear interface.
// Implementation using Scroll.
// List uses Scroll to retrieve records with optional filtering.
func (s *QdrantStore) List(ctx context.Context, filters map[string]interface{}, limit int, offset int) ([]types.Record, error) {
	// Build Filter
	var qdrantFilter *qdrant.Filter
	if len(filters) > 0 {
		var conditions []*qdrant.Condition
		for k, v := range filters {
			valStr := fmt.Sprintf("%v", v)

			// Hack fix for user_id nesting
			key := k
			if k == "user_id" {
				key = "metadata.user_id"
			}

			conditions = append(conditions, &qdrant.Condition{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: key,
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Keyword{
								Keyword: valStr,
							},
						},
					},
				},
			})
		}
		qdrantFilter = &qdrant.Filter{
			Must: conditions,
		}
	}

	// Logic for offset: Qdrant Scroll uses "Offset" as a PointID to start AFTER.
	// It does not support integer offset for skipping N items efficiently.
	// It DOES support an Integer "Offset" in `ScrollPoints` actually?
	// Checking proto definition: `PointId offset = 3;` - It acts as a cursor.
	// However, if we want "Page 2" (skip 50), we have to scroll 50 items and take the last ID as offset.
	// For this implementation, since we need to support integer offset from the API, we will just fetch limit+offset and slice.
	// This is inefficient for deep pages but simple for now.

	var allPoints []*qdrant.RetrievedPoint
	var nextOffset *qdrant.PointId

	// Loop until we have enough to cover the offset+limit
	// Or we can just use search if we had a vector? No.
	// We will try to fetch fetchLimit items.

	// Actually, Qdrant Go client ScrollPoints takes "Offset" which is a PointId.
	// We'll iterate.

	currentCount := 0
	targetCount := limit + offset

	for currentCount < targetCount {
		batchSize := uint32(100) // Fetch in batches
		if uint32(targetCount-currentCount) < batchSize {
			batchSize = uint32(targetCount - currentCount)
		}

		scrollResult, err := s.client.GetPointsClient().Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: s.collection,
			Limit:          &batchSize,
			Offset:         nextOffset,
			WithPayload:    qdrant.NewWithPayload(true),
			Filter:         qdrantFilter,
		})
		if err != nil {
			return nil, err
		}

		if len(scrollResult.Result) == 0 {
			break
		}

		allPoints = append(allPoints, scrollResult.Result...)
		currentCount += len(scrollResult.Result)
		nextOffset = scrollResult.NextPageOffset
		if nextOffset == nil {
			break
		}
	}

	// Now apply offset and limit in memory
	var records []types.Record

	start := offset
	end := offset + limit
	if start >= len(allPoints) {
		return []types.Record{}, nil
	}
	if end > len(allPoints) {
		end = len(allPoints)
	}

	slicedPoints := allPoints[start:end]

	for _, pt := range slicedPoints {
		payload := pt.Payload

		content := ""
		if val, ok := payload["content"]; ok {
			content = val.GetStringValue()
		}

		var ts time.Time
		if val, ok := payload["timestamp"]; ok {
			ts, _ = time.Parse(time.RFC3339, val.GetStringValue())
		}

		typeStr := ""
		if val, ok := payload["type"]; ok {
			typeStr = val.GetStringValue()
		}

		rec := types.Record{
			ID:        pt.Id.GetUuid(),
			Content:   content,
			Type:      types.MemoryType(typeStr),
			Timestamp: ts,
			// Metadata could be extracted here too
		}
		// Extract raw metadata for display
		if val, ok := payload["metadata"]; ok {
			// complex handling needed for struct value
			// For simplicity we skip deep metadata parsing or just dump string
			_ = val
		}
		// Since we didn't parse full metadata in Retrieve/Search either, we stick to core fields for list
		records = append(records, rec)
	}
	return records, nil
}

// Update modifies a record. Qdrant Upsert overwrites.
func (s *QdrantStore) Update(ctx context.Context, record types.Record) error {
	// Re-use Add which does upsert.
	return s.Add(ctx, []types.Record{record})
}

// Get retrieves a record.
func (s *QdrantStore) Get(ctx context.Context, id string) (*types.Record, error) {
	points, err := s.client.GetPointsClient().Get(ctx, &qdrant.GetPoints{
		CollectionName: s.collection,
		Ids: []*qdrant.PointId{
			qdrant.NewIDUUID(id),
		},
		WithPayload: qdrant.NewWithPayload(true),
		WithVectors: qdrant.NewWithVectors(true),
	})
	if err != nil {
		return nil, err
	}

	if len(points.Result) == 0 {
		return nil, fmt.Errorf("not found")
	}

	pt := points.Result[0]
	payload := pt.Payload

	content := ""
	if val, ok := payload["content"]; ok {
		content = val.GetStringValue()
	}

	var ts time.Time
	if val, ok := payload["timestamp"]; ok {
		ts, _ = time.Parse(time.RFC3339, val.GetStringValue())
	}

	typeStr := ""
	if val, ok := payload["type"]; ok {
		typeStr = val.GetStringValue()
	}

	// Extract vectors?
	var emb []float32
	if pt.Vectors != nil {
		if v := pt.Vectors.GetVector(); v != nil {
			emb = v.Data
		}
	}

	rec := types.Record{
		ID:        pt.Id.GetUuid(),
		Content:   content,
		Type:      types.MemoryType(typeStr),
		Timestamp: ts,
		Embedding: emb,
	}

	return &rec, nil
}
