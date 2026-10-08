package ingest

import "testing"

func TestDedupeStagesKeepsTheLastName(t *testing.T) {
	in := []Stage{
		{Name: "Blocking tests", Result: "OLD"},
		{Name: "Quarantine", Result: "SUCCESS"},
		{Name: "Blocking tests", Result: "SUCCESS"},
	}
	out := dedupeStages(in)
	if len(out) != 2 {
		t.Fatalf("len = %d", len(out))
	}
	if out[0].Name != "Blocking tests" || out[0].Result != "SUCCESS" {
		t.Fatalf("first = %+v", out[0])
	}
	if out[1].Name != "Quarantine" {
		t.Fatalf("second = %+v", out[1])
	}
}
