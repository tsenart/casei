//go:build amd64

package casei

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type tripleBucketRebarRow struct {
	id, model, path, regex string
	count                  int
}

func tripleBucketRebarRows(t *testing.T) []tripleBucketRebarRow {
	t.Helper()
	wanted := map[string]bool{
		"curated/01-literal/sherlock-casei-en":               true,
		"curated/01-literal/sherlock-casei-ru":               true,
		"curated/02-literal-alternate/sherlock-casei-en":     true,
		"curated/02-literal-alternate/sherlock-casei-ru":     true,
		"imported/leipzig/tom-sawyer-huckle-fin-insensitive": true,
		"imported/sherlock/name-alt3-casei":                  true,
		"imported/sherlock/name-alt5-casei":                  true,
	}
	data, err := os.ReadFile("audit/rebar/runner/testdata/rows.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows []tripleBucketRebarRow
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 || !wanted[fields[0]] {
			continue
		}
		count, err := strconv.Atoi(fields[3])
		if err != nil {
			t.Fatalf("bad expected count in Rebar row %q: %v", fields[0], err)
		}
		rows = append(rows, tripleBucketRebarRow{
			id: fields[0], model: fields[1], path: fields[2], regex: fields[4], count: count,
		})
	}
	if len(rows) != len(wanted) {
		t.Fatalf("found %d selected Rebar rows, want %d", len(rows), len(wanted))
	}
	return rows
}

// tripleBucketRebarStops counts the first filter survivor at each stop, then
// resumes one start later. Like tripleShuftiProofCount, it observes the
// production skip routine without adding counters to the timed path.
func tripleBucketRebarStops(haystack string, plan *searchPlan) int {
	stops := 0
	for at := 0; at+2 < len(haystack); {
		bucket := plan.tripleBucketFilter()
		var skipped int
		if bucket.usable() && asciiPairVBMIEnabled() {
			skipped = tripleBucketSkipBytes(haystack, at, bucket, &plan.triples.shufti)
		} else {
			skipped = tripleSkipBytes(haystack, at, &plan.triples)
		}
		if skipped == 0 {
			stops++
			at++
			continue
		}
		at += skipped
	}
	return stops
}

func rawByteMultiRebarStops(haystack string, filter *rawByteMultiAnchorFilter) int {
	stops := 0
	for at := 0; at+1 < len(haystack); {
		skipped, candidates := rawByteMultiAnchorSkipBytes(haystack, at, filter)
		at += skipped
		if at+1 >= len(haystack) {
			break
		}
		if candidates != 0 {
			stops++
		}
		if skipped == 0 {
			at++
		}
	}
	return stops
}

// TestTripleBucketRebarEvidence records real-row filter survivors and exact
// Matcher.Each results. Set CASEI_REBAR_HAYSTACKS to the directory prepared by
// audit/rebar/haystacks.sh to run it; ordinary unit tests need no downloads.
func TestTripleBucketRebarEvidence(t *testing.T) {
	if os.Getenv("CASEI_REBAR_HAYSTACKS") == "" {
		t.Skip("set CASEI_REBAR_HAYSTACKS to the pinned Rebar haystacks")
	}
	if !asciiPairVBMIEnabled() {
		t.Skip("the bucket evidence requires AVX-512 VBMI")
	}

	for _, row := range tripleBucketRebarRows(t) {
		haystack, err := os.ReadFile(filepath.Join(os.Getenv("CASEI_REBAR_HAYSTACKS"), row.path))
		if err != nil {
			t.Fatalf("%s: read Rebar haystack: %v", row.id, err)
		}
		matcher := NewMatcher(strings.Split(row.regex, "|"))
		matches, widthBytes := 0, 0
		complete := matcher.Each(string(haystack), func(_ Match, width int) bool {
			matches++
			widthBytes += width
			return true
		})
		if !complete {
			t.Fatalf("%s: Matcher.Each did not finish", row.id)
		}
		got := matches
		if row.model == "count-spans" {
			got = widthBytes
		}
		if got != row.count {
			t.Fatalf("%s: Rebar count=%d, Matcher.Each produced %d", row.id, row.count, got)
		}

		switch row.id {
		case "imported/sherlock/name-alt3-casei":
			bucket := matcher.plan.tripleBucketFilter()
			if matcher.plan.patternCount != 7 || matcher.plan.triples.n != 9 || !matcher.plan.triplesComplete ||
				matcher.plan.rawByteMulti.usable() || !bucket.usable() || bucket.prefixCount() != 7 ||
				!matcher.plan.triples.shufti.usable() {
				t.Fatalf("seven-name route is not the bounded shared bucket: forms=%d bucket=%v prefixes=%d shufti=%v",
					matcher.plan.triples.n, bucket.usable(), bucket.prefixCount(), matcher.plan.triples.shufti.usable())
			}
			if _, ok := matcher.plan.rootBucketEachFilter(string(haystack)); ok {
				t.Fatal("seven-name Rebar Each selected the existing four/five-pattern root iterator")
			}
			baselinePlan := *matcher.plan
			baselinePlan.tripleRoots = nil
			baselinePlan.triples.shufti = tripleShuftiFilter{}
			baseline := &Matcher{patterns: matcher.patterns, plan: &baselinePlan}
			gotMatches, gotWidths, gotComplete := collectTripleBucketEach(matcher, string(haystack))
			wantMatches, wantWidths, wantComplete := collectTripleBucketEach(baseline, string(haystack))
			if gotComplete != wantComplete || !reflect.DeepEqual(gotMatches, wantMatches) || !reflect.DeepEqual(gotWidths, wantWidths) {
				t.Fatalf("name-alt3 bucket differs from the baseline per-form plan: bucket=%+v/%v generic=%+v/%v",
					gotMatches, gotWidths, wantMatches, wantWidths)
			}
			bucketStops := tripleBucketRebarStops(string(haystack), matcher.plan)
			genericStops := tripleBucketRebarStops(string(haystack), &baselinePlan)
			if bucketStops >= genericStops {
				t.Fatalf("name-alt3 bucket stops=%d did not reduce baseline per-form stops=%d", bucketStops, genericStops)
			}
			t.Logf("%s: bucket prefixes=%d, bucket stops=%d, baseline per-form stops=%d, exact Each matches=%d, width bytes=%d",
				row.id, bucket.prefixCount(), bucketStops, genericStops, matches, widthBytes)
		case "curated/02-literal-alternate/sherlock-casei-en",
			"imported/leipzig/tom-sawyer-huckle-fin-insensitive",
			"imported/sherlock/name-alt5-casei":
			if matcher.plan.rawByteMulti.usable() || !matcher.plan.tripleBucketFilter().usable() {
				t.Fatalf("%s: expected the plan-owned bucket route", row.id)
			}
			shufti := *matcher.plan
			shufti.tripleRoots = nil
			bucketStops := tripleBucketRebarStops(string(haystack), matcher.plan)
			shuftiStops := tripleBucketRebarStops(string(haystack), &shufti)
			t.Logf("%s: bucket filter stops=%d, Shufti stops=%d, exact Each matches=%d, width bytes=%d",
				row.id, bucketStops, shuftiStops, matches, widthBytes)
			if bucketStops >= shuftiStops {
				t.Fatalf("%s: bucket stops %d did not reduce Shufti stops %d", row.id, bucketStops, shuftiStops)
			}
		case "curated/02-literal-alternate/sherlock-casei-ru":
			if !matcher.plan.rawByteMulti.usable() || matcher.plan.tripleBucketFilter() != nil {
				t.Fatal("Russian counterpart no longer stays on its raw multi-anchor route")
			}
			t.Logf("%s: raw multi-anchor filter stops=%d, exact Each matches=%d, width bytes=%d",
				row.id, rawByteMultiRebarStops(string(haystack), &matcher.plan.rawByteMulti), matches, widthBytes)
		case "curated/01-literal/sherlock-casei-en", "curated/01-literal/sherlock-casei-ru":
			if matcher.plan.tripleBucketFilter() != nil {
				t.Fatalf("%s: single-literal plan unexpectedly compiled a bucket", row.id)
			}
			t.Logf("%s: bucket disabled, exact Each matches=%d, width bytes=%d", row.id, matches, widthBytes)
		}
	}
}
