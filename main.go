package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	inputPath := flag.String("input", "", "path to a creator export JSON file")
	bucket := flag.String("bucket", "creator-nightly-snapshots", "destination bucket")
	date := flag.String("date", "", "snapshot date in YYYY-MM-DD; defaults to today in UTC")
	flag.Parse()

	if *inputPath == "" {
		exitf("-input is required")
	}
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		exitf("INFRAI_API_KEY is required")
	}

	now := time.Now().UTC()
	if *date != "" {
		parsed, err := time.Parse(time.DateOnly, *date)
		if err != nil {
			exitf("parse -date: %v", err)
		}
		now = parsed
	}

	file, err := os.Open(*inputPath)
	if err != nil {
		exitf("open input: %v", err)
	}
	defer file.Close()

	var input CreatorExport
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		exitf("decode input: %v", err)
	}
	plan, err := PlanSnapshot(input, now)
	if err != nil {
		exitf("plan snapshot: %v", err)
	}

	client := NewInfraiClient(apiKey)
	if err := client.storeSnapshot(context.Background(), *bucket, plan); err != nil {
		exitf("store snapshot: %v", err)
	}
	fmt.Printf("stored %s sha256=%s assets=%d subscribers=%d jobs=%d\n",
		plan.Key,
		plan.SHA256,
		plan.RecordCounts["digital_assets"],
		plan.RecordCounts["subscriber_updates"],
		plan.RecordCounts["content_processing"],
	)
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
