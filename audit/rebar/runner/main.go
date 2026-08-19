package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/tsenart/casei"
)

type config struct {
	name            string
	model           string
	patterns        []string
	haystack        string
	caseInsensitive bool
	maxIters        uint64
	maxWarmupIters  uint64
	maxTime         time.Duration
	maxWarmupTime   time.Duration
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "version" {
		fmt.Println("casei-rebar 1")
		return nil
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	c, err := readConfig(raw)
	if err != nil {
		return err
	}
	if !c.caseInsensitive {
		return errors.New("casei runner only accepts case-insensitive benchmarks")
	}
	if c.model != "count" && c.model != "count-spans" {
		return fmt.Errorf("unsupported model %q", c.model)
	}
	patterns, err := literalAlternation(c.patterns)
	if err != nil {
		return err
	}
	matcher := casei.NewMatcher(patterns)
	bench := func() (int, error) {
		return countMatches(c.haystack, patterns, matcher, c.model == "count-spans")
	}

	warmupStart := time.Now()
	for i := uint64(0); i < c.maxWarmupIters; i++ {
		if _, err := bench(); err != nil {
			return err
		}
		if time.Since(warmupStart) >= c.maxWarmupTime {
			break
		}
	}

	out := bufio.NewWriter(os.Stdout)
	runStart := time.Now()
	for i := uint64(0); i < c.maxIters; i++ {
		start := time.Now()
		count, err := bench()
		duration := time.Since(start)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%d,%d\n", duration.Nanoseconds(), count)
		if time.Since(runStart) >= c.maxTime {
			break
		}
	}
	return out.Flush()
}

func countMatches(haystack string, patterns []string, matcher *casei.Matcher, spans bool) (int, error) {
	total := 0
	for at := 0; at <= len(haystack); {
		match, ok := matcher.Find(haystack[at:])
		if !ok {
			break
		}
		start := at + match.Start
		width, ok := foldPrefixWidth(haystack[start:], patterns[match.Pattern])
		if !ok || width == 0 {
			return 0, fmt.Errorf("casei returned an unverifiable match at %d for pattern %q", start, patterns[match.Pattern])
		}
		if spans {
			total += width
		} else {
			total++
		}
		at = start + width
	}
	return total, nil
}

func foldPrefixWidth(haystack, pattern string) (int, bool) {
	consumed := 0
	for len(pattern) > 0 {
		pr, pn := utf8.DecodeRuneInString(pattern)
		if len(haystack) == consumed {
			return 0, false
		}
		hr, hn := utf8.DecodeRuneInString(haystack[consumed:])
		if pr == utf8.RuneError && pn == 1 {
			if hr != utf8.RuneError || hn != 1 || pattern[0] != haystack[consumed] {
				return 0, false
			}
			pattern = pattern[1:]
			consumed++
			continue
		}
		if hr == utf8.RuneError && hn == 1 || !foldEqual(pr, hr) {
			return 0, false
		}
		pattern = pattern[pn:]
		consumed += hn
	}
	return consumed, true
}

func foldEqual(a, b rune) bool {
	if a == b {
		return true
	}
	for r := unicode.SimpleFold(a); r != a; r = unicode.SimpleFold(r) {
		if r == b {
			return true
		}
	}
	return false
}

func literalAlternation(raw []string) ([]string, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("expected one rebar regex, got %d", len(raw))
	}
	if strings.ContainsAny(raw[0], `\\[](){}.*+?^$`) {
		return nil, fmt.Errorf("not a literal alternation: %q", raw[0])
	}
	patterns := strings.Split(raw[0], "|")
	for _, pattern := range patterns {
		if pattern == "" {
			return nil, errors.New("empty alternation is unsupported")
		}
	}
	return patterns, nil
}

func readConfig(raw []byte) (config, error) {
	var c config
	for len(raw) > 0 {
		keyEnd := bytes.IndexByte(raw, ':')
		if keyEnd < 0 {
			return c, errors.New("invalid KLV key")
		}
		lengthEnd := bytes.IndexByte(raw[keyEnd+1:], ':')
		if lengthEnd < 0 {
			return c, errors.New("invalid KLV length")
		}
		lengthEnd += keyEnd + 1
		n, err := strconv.Atoi(string(raw[keyEnd+1 : lengthEnd]))
		if err != nil || n < 0 || lengthEnd+1+n >= len(raw) || raw[lengthEnd+1+n] != '\n' {
			return c, errors.New("invalid KLV value")
		}
		key := string(raw[:keyEnd])
		value := string(raw[lengthEnd+1 : lengthEnd+1+n])
		raw = raw[lengthEnd+1+n+1:]
		switch key {
		case "name":
			c.name = value
		case "model":
			c.model = value
		case "pattern":
			c.patterns = append(c.patterns, value)
		case "haystack":
			c.haystack = value
		case "case-insensitive":
			c.caseInsensitive, err = strconv.ParseBool(value)
		case "unicode":
			// casei always applies Unicode simple folding. The audit separately
			// records which rebar rows request ASCII-only case insensitivity.
		case "max-iters":
			c.maxIters, err = strconv.ParseUint(value, 10, 64)
		case "max-warmup-iters":
			c.maxWarmupIters, err = strconv.ParseUint(value, 10, 64)
		case "max-time":
			var nanos uint64
			nanos, err = strconv.ParseUint(value, 10, 64)
			c.maxTime = time.Duration(nanos)
		case "max-warmup-time":
			var nanos uint64
			nanos, err = strconv.ParseUint(value, 10, 64)
			c.maxWarmupTime = time.Duration(nanos)
		default:
			return c, fmt.Errorf("unrecognized KLV key %q", key)
		}
		if err != nil {
			return c, fmt.Errorf("invalid %s: %w", key, err)
		}
	}
	return c, nil
}
