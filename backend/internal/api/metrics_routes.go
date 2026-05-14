package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"memobase/backend/internal/core"
	"memobase/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func init() {
	Register(&prometheusProxyRegistrar{})
}

type prometheusProxyRegistrar struct{}

func (prometheusProxyRegistrar) Register(public *gin.RouterGroup, _ *gin.RouterGroup, _ *core.App) {
	public.GET("/metrics/prometheus", handlePrometheusProxy())
}

func prometheusURL() string {
	if u := os.Getenv("PROMETHEUS_URL"); u != "" {
		return u
	}
	return "http://prometheus:9090"
}

func handlePrometheusProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("query")
		if query == "" {
			util.BadRequest(c, "MISSING_QUERY", "query parameter is required", nil)
			return
		}

		start := c.DefaultQuery("start", "")
		end := c.DefaultQuery("end", "")
		step := c.DefaultQuery("step", "60")

		base := prometheusURL()
		endpoint := "/api/v1/query_range"

		params := url.Values{}
		params.Set("query", query)
		params.Set("step", step)
		if start != "" {
			params.Set("start", start)
		}
		if end != "" {
			params.Set("end", end)
		}

		reqURL := fmt.Sprintf("%s%s?%s", base, endpoint, params.Encode())

		resp, err := http.Get(reqURL)
		if err != nil {
			util.Internal(c, fmt.Sprintf("prometheus request failed: %v", err))
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			util.Internal(c, fmt.Sprintf("failed to read prometheus response: %v", err))
			return
		}

		if resp.StatusCode != http.StatusOK {
			util.Internal(c, fmt.Sprintf("prometheus returned %d", resp.StatusCode))
			return
		}

		var promResp prometheusResponse
		if err := json.Unmarshal(body, &promResp); err != nil {
			util.Internal(c, "failed to parse prometheus response")
			return
		}

		if promResp.Status != "success" {
			util.Internal(c, fmt.Sprintf("prometheus query failed: %s", promResp.Error))
			return
		}

		// Transform to a simpler format for the frontend
		results := make([]prometheusSeries, 0, len(promResp.Data.Result))
		for _, r := range promResp.Data.Result {
			values := make([][2]interface{}, 0, len(r.Values))
			for _, v := range r.Values {
				if len(v) == 2 {
					values = append(values, [2]interface{}{v[0], v[1]})
				}
			}
			label := ""
			if r.Metric != nil {
				if l, ok := r.Metric["__name__"]; ok {
					label = l
				}
				// Use first non-__name__ label as legend
				for k, v := range r.Metric {
					if k != "__name__" {
						if label != "" {
							label += " "
						}
						label += k + "=" + v
						break
					}
				}
			}
			results = append(results, prometheusSeries{
				Label:  label,
				Values: values,
			})
		}

		util.Success(c, http.StatusOK, gin.H{"series": results})
	}
}

type prometheusResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   struct {
		ResultType string            `json:"resultType"`
		Result     []prometheusResult `json:"result"`
	} `json:"data"`
}

type prometheusResult struct {
	Metric map[string]string `json:"metric"`
	Values []json.RawMessage `json:"values"`
}

type prometheusSeries struct {
	Label  string            `json:"label"`
	Values [][2]interface{}  `json:"values"`
}
