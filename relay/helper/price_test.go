package helper

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelPriceHelperTieredUsesPreloadedRequestInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"billing_setting.billing_mode": `{"tiered-test-model":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"tiered-test-model":"param(\"stream\") == true ? tier(\"stream\", p * 3) : tier(\"base\", p * 2)"}`,
	}))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/api/channel/test/1", nil)
	req.Body = nil
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req
	ctx.Set("group", "default")

	info := &relaycommon.RelayInfo{
		OriginModelName: "tiered-test-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		BillingRequestInput: &billingexpr.RequestInput{
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"stream":true}`),
		},
	}

	priceData, err := ModelPriceHelper(ctx, info, 1000, &types.TokenCountMeta{})
	require.NoError(t, err)
	require.Equal(t, 1500, priceData.QuotaToPreConsume)
	require.NotNil(t, info.TieredBillingSnapshot)
	require.Equal(t, "stream", info.TieredBillingSnapshot.EstimatedTier)
	require.Equal(t, billing_setting.BillingModeTieredExpr, info.TieredBillingSnapshot.BillingMode)
	require.Equal(t, common.QuotaPerUnit, info.TieredBillingSnapshot.QuotaPerUnit)
}

// The request-rule editor emits one factor per rule group, e.g.
// `(param("x") != nil && param("x") > 4 ? 2 : 1)`, multiplied onto the base
// price. Every comparison operator must apply the multiplier on the per-call
// (task) path exactly like equality does.
func TestModelPriceHelperPerCallAppliesRequestRuleMultiplier(t *testing.T) {
	gin.SetMode(gin.TestMode)

	saved := map[string]string{}
	require.NoError(t, config.GlobalConfig.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(saved))
	})

	const videoBody = `{"model":"veo","metadata":{"durationSeconds":8}}`
	tests := []struct {
		name           string
		rule           string
		body           string
		wantMultiplier float64
	}{
		{"equals", `param("metadata.durationSeconds") == 8`, videoBody, 2},
		{"greater than", `param("metadata.durationSeconds") != nil && param("metadata.durationSeconds") > 4`, videoBody, 2},
		{"greater than or equal", `param("metadata.durationSeconds") != nil && param("metadata.durationSeconds") >= 8`, videoBody, 2},
		{"less than", `param("metadata.durationSeconds") != nil && param("metadata.durationSeconds") < 10`, videoBody, 2},
		{"less than or equal", `param("metadata.durationSeconds") != nil && param("metadata.durationSeconds") <= 8`, videoBody, 2},
		{"greater than at the boundary", `param("metadata.durationSeconds") != nil && param("metadata.durationSeconds") > 8`, videoBody, 1},
		{"comparison on an absent param", `param("metadata.durationSeconds") != nil && param("metadata.durationSeconds") > 4`, `{"model":"veo"}`, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const modelName = "per-call-rule-model"
			billingMode, err := common.Marshal(map[string]string{modelName: billing_setting.BillingModeTieredExpr})
			require.NoError(t, err)
			// $0.5 per request (expression unit is price * 1_000_000), then the rule factor.
			billingExpr, err := common.Marshal(map[string]string{
				modelName: `(tier("base", 500000)) * (` + tt.rule + ` ? 2 : 1)`,
			})
			require.NoError(t, err)
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
				"billing_setting.billing_mode": string(billingMode),
				"billing_setting.billing_expr": string(billingExpr),
			}))

			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", io.NopCloser(bytes.NewReader([]byte(tt.body))))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set("group", "default")
			ctx.Set(common.KeyRequestBody, []byte(tt.body))

			info := &relaycommon.RelayInfo{
				OriginModelName: modelName,
				UserGroup:       "default",
				UsingGroup:      "default",
			}

			priceData, err := ModelPriceHelperPerCall(ctx, info)
			require.NoError(t, err)
			require.Equal(t, int(0.5*common.QuotaPerUnit*tt.wantMultiplier), priceData.Quota)
		})
	}
}
