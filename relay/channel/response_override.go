package channel

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

const responseOverrideChunkSize = 32 * 1024

type compiledResponseOverrideRule struct {
	match   []byte
	replace []byte
}

// applyResponseOverride rewrites the upstream response body in place using the
// channel's response_override rules. It wraps the body with a streaming reader
// so replacements also work for SSE and other chunked responses, and drops the
// Content-Length header because the rewritten size is no longer known.
func applyResponseOverride(resp *http.Response, info *relaycommon.RelayInfo) {
	if resp == nil || resp.Body == nil || info == nil || info.ChannelMeta == nil {
		return
	}
	rules := info.ChannelOtherSettings.ResponseOverride
	if len(rules) == 0 {
		return
	}
	// The HTTP transport only decompresses transparently when it added
	// Accept-Encoding itself. A still-encoded body cannot be safely rewritten.
	if encoding := strings.TrimSpace(resp.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return
	}
	// Binary payloads (audio, images, archives) must never be rewritten: a
	// short keyword can otherwise corrupt bytes inside the payload.
	if !responseOverrideRewritableContentType(resp.Header.Get("Content-Type")) {
		return
	}

	active := make([]compiledResponseOverrideRule, 0, len(rules))
	var requestParams map[string]any
	requestParamsLoaded := false
	for _, rule := range rules {
		if rule.Match == "" || rule.Match == rule.Replace {
			continue
		}
		if rule.RequestParam != "" {
			if !requestParamsLoaded {
				requestParams = responseOverrideRequestParams(info.Request)
				requestParamsLoaded = true
			}
			if !responseOverrideConditionMatches(requestParams, rule.RequestParam, rule.RequestValue) {
				continue
			}
		}
		active = append(active, compiledResponseOverrideRule{
			match:   []byte(rule.Match),
			replace: []byte(rule.Replace),
		})
	}
	if len(active) == 0 {
		return
	}

	resp.Body = newResponseOverrideReader(resp.Body, active)
	resp.ContentLength = -1
	resp.Header.Del("Content-Length")
}

func responseOverrideRewritableContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" {
		return true
	}
	return strings.Contains(contentType, "json") || strings.HasPrefix(contentType, "text/")
}

func responseOverrideRequestParams(request dto.Request) map[string]any {
	if request == nil {
		return nil
	}
	data, err := common.Marshal(request)
	if err != nil {
		return nil
	}
	var params map[string]any
	if err := common.Unmarshal(data, &params); err != nil {
		return nil
	}
	return params
}

func responseOverrideConditionMatches(params map[string]any, path string, expected string) bool {
	value, ok := responseOverrideLookupParam(params, path)
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case nil:
		return expected == "null"
	case string:
		return typed == expected
	case bool:
		return strconv.FormatBool(typed) == expected
	case float64:
		expectedNumber, err := strconv.ParseFloat(strings.TrimSpace(expected), 64)
		return err == nil && typed == expectedNumber
	default:
		encoded, err := common.Marshal(typed)
		return err == nil && strings.TrimSpace(string(encoded)) == strings.TrimSpace(expected)
	}
}

func responseOverrideLookupParam(params map[string]any, path string) (any, bool) {
	if params == nil {
		return nil, false
	}
	var current any = params
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return nil, false
		}
		switch node := current.(type) {
		case map[string]any:
			value, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}

type responseOverrideReader struct {
	source     io.ReadCloser
	rules      []compiledResponseOverrideRule
	maxMatch   int
	readBuffer []byte
	pending    []byte
	emit       []byte
	sourceErr  error
	eof        bool
}

func newResponseOverrideReader(source io.ReadCloser, rules []compiledResponseOverrideRule) io.ReadCloser {
	maxMatch := 0
	for _, rule := range rules {
		if len(rule.match) > maxMatch {
			maxMatch = len(rule.match)
		}
	}
	return &responseOverrideReader{
		source:   source,
		rules:    rules,
		maxMatch: maxMatch,
	}
}

func (r *responseOverrideReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(r.emit) == 0 {
		if r.sourceErr != nil {
			return 0, r.sourceErr
		}
		if r.eof {
			return 0, io.EOF
		}
		if r.readBuffer == nil {
			r.readBuffer = make([]byte, responseOverrideChunkSize)
		}
		count, err := r.source.Read(r.readBuffer)
		if count > 0 {
			r.pending = append(r.pending, r.readBuffer[:count]...)
			r.process(false)
		}
		if err != nil {
			// Flush whatever is still buffered before surfacing the error.
			r.process(true)
			r.eof = true
			if !errors.Is(err, io.EOF) {
				r.sourceErr = err
			}
		}
	}
	count := copy(p, r.emit)
	r.emit = r.emit[count:]
	return count, nil
}

func (r *responseOverrideReader) Close() error {
	return r.source.Close()
}

// pendingHoldback returns the length of the longest pending suffix that is a
// prefix of some keyword. Only that suffix can still grow into a match, so all
// other bytes can be emitted immediately (important for SSE latency).
func (r *responseOverrideReader) pendingHoldback() int {
	maxLength := r.maxMatch - 1
	if len(r.pending) < maxLength {
		maxLength = len(r.pending)
	}
	hold := 0
	for _, rule := range r.rules {
		match := rule.match
		limit := len(match) - 1
		if limit > maxLength {
			limit = maxLength
		}
		for length := limit; length > hold; length-- {
			if bytes.Equal(r.pending[len(r.pending)-length:], match[:length]) {
				hold = length
				break
			}
		}
	}
	return hold
}

// process applies every rule to the pending buffer and moves the output into
// emit. A suffix that could still grow into a longer match is held back unless
// final is set.
func (r *responseOverrideReader) process(final bool) {
	if len(r.pending) == 0 {
		return
	}
	safe := len(r.pending)
	if !final {
		safe -= r.pendingHoldback()
	}

	output := make([]byte, 0, len(r.pending))
	position := 0
	for position < safe {
		bestIndex := -1
		bestLength := 0
		bestRule := -1
		for index := range r.rules {
			match := r.rules[index].match
			found := bytes.Index(r.pending[position:], match)
			if found < 0 {
				continue
			}
			found += position
			// A match starting in the held-back tail may still grow into a
			// longer keyword, so only accept matches that start before it.
			if found >= safe {
				continue
			}
			if bestIndex < 0 || found < bestIndex || (found == bestIndex && len(match) > bestLength) {
				bestIndex = found
				bestLength = len(match)
				bestRule = index
			}
		}
		if bestIndex < 0 {
			break
		}
		output = append(output, r.pending[position:bestIndex]...)
		output = append(output, r.rules[bestRule].replace...)
		position = bestIndex + bestLength
	}

	emitEnd := safe
	if position > emitEnd {
		emitEnd = position
	}
	if emitEnd > len(r.pending) {
		emitEnd = len(r.pending)
	}
	output = append(output, r.pending[position:emitEnd]...)

	r.emit = append(r.emit, output...)
	r.pending = append(r.pending[:0], r.pending[emitEnd:]...)
}
