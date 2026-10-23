package helper

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func ResolveIncomingBillingExprRequestInput(c *gin.Context, info *relaycommon.RelayInfo) (billingexpr.RequestInput, error) {
	if info != nil && info.BillingRequestInput != nil {
		input := cloneRequestInput(*info.BillingRequestInput)
		merged := cloneStringMap(info.RequestHeaders)
		for k, v := range input.Headers {
			merged[k] = v
		}
		input.Headers = merged
		return input, nil
	}

	input := billingexpr.RequestInput{}
	if info != nil {
		input.Headers = cloneStringMap(info.RequestHeaders)
	}

	bodyBytes, err := readIncomingBillingExprBody(c)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	input.Body = bodyBytes
	return input, nil
}

func BuildBillingExprRequestInputFromRequest(request dto.Request, headers map[string]string) (billingexpr.RequestInput, error) {
	input := billingexpr.RequestInput{
		Headers: cloneStringMap(headers),
	}
	if request == nil {
		return input, nil
	}

	bodyBytes, err := common.Marshal(request)
	if err != nil {
		return billingexpr.RequestInput{}, err
	}
	input.Body = bodyBytes
	return input, nil
}

func readIncomingBillingExprBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil {
		return nil, nil
	}
	contentType := c.Request.Header.Get("Content-Type")

	// JSON body – straightforward.
	if isJSONContentType(contentType) {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return nil, err
		}
		return storage.Bytes()
	}

	// Multipart form – convert text fields into a lightweight JSON object so
	// that the billing expression engine's param() function can query them.
	if isMultipartContentType(contentType) {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, err
		}
		// Build a JSON object from the text fields (skip file fields).
		kv := make(map[string]string, len(form.Value))
		for field, values := range form.Value {
			if len(values) > 0 {
				kv[field] = values[0]
			}
		}
		if len(kv) == 0 {
			return nil, nil
		}
		return common.Marshal(kv)
	}

	return nil, nil
}

func isMultipartContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func cloneRequestInput(src billingexpr.RequestInput) billingexpr.RequestInput {
	input := billingexpr.RequestInput{
		Headers: cloneStringMap(src.Headers),
	}
	if len(src.Body) > 0 {
		input.Body = append([]byte(nil), src.Body...)
	}
	return input
}

func isJSONContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "application/json")
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		if strings.TrimSpace(key) == "" {
			continue
		}
		dst[key] = value
	}
	return dst
}
