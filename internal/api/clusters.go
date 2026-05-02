package api

import (
	"encoding/json"
	"net/http"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/cluster"
)

type clustersHandler struct {
	registry cluster.Registry
}

func (h *clustersHandler) list(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := types.ClustersResponse{Clusters: []types.Cluster{}}
	if h.registry != nil {
		for _, e := range h.registry.List() {
			resp.Clusters = append(resp.Clusters, toClusterDTO(e))
		}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func toClusterDTO(e *cluster.Entry) types.Cluster {
	st := e.Status()
	return types.Cluster{
		ID:            e.Name,
		Name:          e.DisplayName,
		Region:        e.Region,
		Env:           e.Environment,
		Status:        clusterHealthStatus(st),
		Apps:          st.Summary.Apps,
		Ready:         st.Summary.Ready,
		Failing:       st.Summary.Failing,
		Suspended:     st.Summary.Suspended,
		Sources:       st.Summary.Sources,
		Version:       st.FluxVersion,
		Reachable:     st.Reachable,
		FluxInstalled: st.FluxInstalled,
		LastError:     st.LastError,
	}
}

// clusterHealthStatus collapses Reachable / FluxInstalled / failing-count
// into the four-bucket status the Fleet UI renders.
func clusterHealthStatus(s cluster.Status) string {
	switch {
	case !s.Reachable:
		return "unreachable"
	case !s.FluxInstalled:
		return "degraded"
	case s.Summary.Failing > 0:
		return "failing"
	case s.Summary.Suspended > 0 && s.Summary.Failing == 0:
		return "healthy"
	default:
		return "healthy"
	}
}
