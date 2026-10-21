package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type CreatorExport struct {
	CreatorID        string             `json:"creator_id"`
	DigitalAssets    []DigitalAsset     `json:"digital_assets"`
	SubscriberEvents []SubscriberUpdate `json:"subscriber_updates"`
	ContentJobs      []ContentJob       `json:"content_processing"`
}

type DigitalAsset struct {
	ID          string `json:"id"`
	DeliveryKey string `json:"delivery_key"`
	MediaType   string `json:"media_type"`
}

type SubscriberUpdate struct {
	SubscriberID string `json:"subscriber_id"`
	State        string `json:"state"`
	ChangedAt    string `json:"changed_at"`
}

type ContentJob struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
}

type Snapshot struct {
	SchemaVersion int           `json:"schema_version"`
	SnapshotDate  string        `json:"snapshot_date"`
	Payload       CreatorExport `json:"payload"`
}

type SnapshotPlan struct {
	Key          string
	Bytes        []byte
	SHA256       string
	RecordCounts map[string]int
}

func PlanSnapshot(input CreatorExport, now time.Time) (SnapshotPlan, error) {
	if input.CreatorID == "" {
		return SnapshotPlan{}, fmt.Errorf("creator_id is required")
	}

	snapshot := Snapshot{
		SchemaVersion: 1,
		SnapshotDate:  now.UTC().Format(time.DateOnly),
		Payload:       input,
	}
	body, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return SnapshotPlan{}, fmt.Errorf("encode snapshot: %w", err)
	}
	body = append(body, '\n')
	digest := sha256.Sum256(body)

	return SnapshotPlan{
		Key:    fmt.Sprintf("creators/%s/%s.json", input.CreatorID, snapshot.SnapshotDate),
		Bytes:  body,
		SHA256: hex.EncodeToString(digest[:]),
		RecordCounts: map[string]int{
			"digital_assets":     len(input.DigitalAssets),
			"subscriber_updates": len(input.SubscriberEvents),
			"content_processing": len(input.ContentJobs),
		},
	}, nil
}
