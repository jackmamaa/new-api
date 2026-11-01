package channel

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chunkedReadCloser struct {
	data      []byte
	chunkSize int
}

func (r *chunkedReadCloser) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	size := r.chunkSize
	if size > len(p) {
		size = len(p)
	}
	if size > len(r.data) {
		size = len(r.data)
	}
	copy(p, r.data[:size])
	r.data = r.data[size:]
	return size, nil
}

func (r *chunkedReadCloser) Close() error {
	return nil
}

func TestResponseOverrideReaderReplacesAcrossChunkBoundaries(t *testing.T) {
	t.Parallel()

	original := `{"background":"auto","created":1793525585,"data":[{"url":"https://test.png"}],"model":"gpt-image-2.5-flare","moderation":"auto","output_format":"png","quality":"low","size":"1024x1024"}`
	expected := strings.ReplaceAll(original, "gpt-image-2.5-flare", "gpt-image-2.5-sunburst")

	reader := newResponseOverrideReader(
		&chunkedReadCloser{data: []byte(original), chunkSize: 3},
		[]compiledResponseOverrideRule{{
			match:   []byte("gpt-image-2.5-flare"),
			replace: []byte("gpt-image-2.5-sunburst"),
		}},
	)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, expected, string(got))
}

func TestResponseOverrideReaderPrefersLongestKeywordAtSamePosition(t *testing.T) {
	t.Parallel()

	reader := newResponseOverrideReader(
		&chunkedReadCloser{data: []byte("xhigh-high"), chunkSize: 2},
		[]compiledResponseOverrideRule{
			{match: []byte("high"), replace: []byte("low")},
			{match: []byte("xhigh"), replace: []byte("max")},
		},
	)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "max-low", string(got))
}

func TestResponseOverrideReaderWaitsForLongerKeyword(t *testing.T) {
	t.Parallel()

	reader := newResponseOverrideReader(
		&chunkedReadCloser{data: []byte("zabcdef"), chunkSize: 4},
		[]compiledResponseOverrideRule{
			{match: []byte("abc"), replace: []byte("X")},
			{match: []byte("abcdef"), replace: []byte("Y")},
		},
	)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "zY", string(got))
}

func TestResponseOverrideReaderDoesNotHoldUnrelatedTail(t *testing.T) {
	t.Parallel()

	payload := "data: hello\n\n"
	reader := newResponseOverrideReader(
		&chunkedReadCloser{data: []byte(payload), chunkSize: len(payload)},
		[]compiledResponseOverrideRule{{match: []byte("xhigh"), replace: []byte("max")}},
	)
	defer reader.Close()

	buffer := make([]byte, len(payload))
	count, err := reader.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, payload, string(buffer[:count]))
}

type nestedOverrideRequest struct {
	dto.BaseRequest
	Quality  string         `json:"quality,omitempty"`
	Count    int            `json:"n,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func TestApplyResponseOverrideRequestConditions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  dto.Request
		rules    []dto.ResponseOverrideRule
		expected string
	}{
		{
			name:    "condition matches",
			request: &nestedOverrideRequest{Quality: "max"},
			rules: []dto.ResponseOverrideRule{{
				Match:        "xhigh",
				Replace:      "max",
				RequestParam: "quality",
				RequestValue: "max",
			}},
			expected: `{"quality":"max"}`,
		},
		{
			name:    "condition does not match",
			request: &nestedOverrideRequest{Quality: "high"},
			rules: []dto.ResponseOverrideRule{{
				Match:        "xhigh",
				Replace:      "max",
				RequestParam: "quality",
				RequestValue: "max",
			}},
			expected: `{"quality":"xhigh"}`,
		},
		{
			name:    "nested condition matches",
			request: &nestedOverrideRequest{Metadata: map[string]any{"quality": "max"}},
			rules: []dto.ResponseOverrideRule{{
				Match:        "xhigh",
				Replace:      "max",
				RequestParam: "metadata.quality",
				RequestValue: "max",
			}},
			expected: `{"quality":"max"}`,
		},
		{
			name:    "numeric condition matches",
			request: &nestedOverrideRequest{Count: 2},
			rules: []dto.ResponseOverrideRule{{
				Match:        "xhigh",
				Replace:      "max",
				RequestParam: "n",
				RequestValue: "2",
			}},
			expected: `{"quality":"max"}`,
		},
		{
			name:    "unconditional rule always applies",
			request: &nestedOverrideRequest{Quality: "high"},
			rules: []dto.ResponseOverrideRule{{
				Match:   "xhigh",
				Replace: "max",
			}},
			expected: `{"quality":"max"}`,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				Request: test.request,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelOtherSettings: dto.ChannelOtherSettings{ResponseOverride: test.rules},
				},
			}
			resp := &http.Response{
				Header:        http.Header{"Content-Length": []string{"18"}},
				ContentLength: 18,
				Body:          io.NopCloser(strings.NewReader(`{"quality":"xhigh"}`)),
			}

			applyResponseOverride(resp, info)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.JSONEq(t, test.expected, string(body))
		})
	}
}

func TestApplyResponseOverrideDropsContentLength(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ResponseOverride: []dto.ResponseOverrideRule{{Match: "flare", Replace: "sunburst"}},
			},
		},
	}
	resp := &http.Response{
		Header:        http.Header{"Content-Length": []string{"8"}},
		ContentLength: 8,
		Body:          io.NopCloser(strings.NewReader("flare")),
	}

	applyResponseOverride(resp, info)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "sunburst", string(body))
	assert.Equal(t, int64(-1), resp.ContentLength)
	assert.Empty(t, resp.Header.Get("Content-Length"))
}

func TestApplyResponseOverrideSkipsBinaryBodies(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ResponseOverride: []dto.ResponseOverrideRule{{Match: "flare", Replace: "sunburst"}},
			},
		},
	}
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"audio/mpeg"}},
		Body:   io.NopCloser(strings.NewReader("flare")),
	}

	applyResponseOverride(resp, info)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "flare", string(body))
}

func TestApplyResponseOverrideSkipsEncodedBodies(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				ResponseOverride: []dto.ResponseOverrideRule{{Match: "flare", Replace: "sunburst"}},
			},
		},
	}
	resp := &http.Response{
		Header: http.Header{"Content-Encoding": []string{"gzip"}},
		Body:   io.NopCloser(strings.NewReader("flare")),
	}

	applyResponseOverride(resp, info)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "flare", string(body))
}
