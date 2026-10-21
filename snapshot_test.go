package main

import (
	"strings"
	"testing"
	"time"
)

func TestPlanSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		input      CreatorExport
		wantKey    string
		wantCounts map[string]int
		wantErr    string
	}{
		{
			name: "groups commerce records under the creator and UTC date",
			input: CreatorExport{
				CreatorID: "maker-42",
				DigitalAssets: []DigitalAsset{
					{ID: "asset-1", DeliveryKey: "downloads/course.zip", MediaType: "application/zip"},
				},
				SubscriberEvents: []SubscriberUpdate{
					{SubscriberID: "sub-7", State: "active", ChangedAt: "2026-09-25T23:58:00Z"},
					{SubscriberID: "sub-8", State: "paused", ChangedAt: "2026-09-25T23:59:00Z"},
				},
				ContentJobs: []ContentJob{
					{ID: "job-3", Kind: "transcode", Status: "complete"},
				},
			},
			wantKey: "creators/maker-42/2026-09-26.json",
			wantCounts: map[string]int{
				"digital_assets": 1, "subscriber_updates": 2, "content_processing": 1,
			},
		},
		{name: "rejects an export without ownership", input: CreatorExport{}, wantErr: "creator_id is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := PlanSnapshot(tt.input, time.Date(2026, 9, 26, 9, 30, 0, 0, time.FixedZone("east", 8*60*60)))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("PlanSnapshot() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PlanSnapshot() error = %v", err)
			}
			if plan.Key != tt.wantKey {
				t.Fatalf("key = %q, want %q", plan.Key, tt.wantKey)
			}
			for field, want := range tt.wantCounts {
				if got := plan.RecordCounts[field]; got != want {
					t.Errorf("count %s = %d, want %d", field, got, want)
				}
			}
			if len(plan.SHA256) != 64 {
				t.Errorf("digest length = %d, want 64", len(plan.SHA256))
			}
		})
	}
}
