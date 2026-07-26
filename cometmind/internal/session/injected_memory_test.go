package session

import "testing"

func TestUnmarshalInjectedMemoriesBackfillsHistoricalBucket(t *testing.T) {
	got := unmarshalInjectedMemories(`[
		{"id":"p","content":"pref","kind":"preference","similarity":0,"effective_weight":1},
		{"id":"t","content":"task","kind":"task_summary","similarity":0,"effective_weight":1},
		{"id":"f","content":"fact","kind":"fact","similarity":0,"effective_weight":1}
	]`)
	if len(got) != 3 || got[0].Bucket != "preference" || got[1].Bucket != "task_outcome" || got[2].Bucket != "semantic" {
		t.Fatalf("buckets = %#v", got)
	}
}
