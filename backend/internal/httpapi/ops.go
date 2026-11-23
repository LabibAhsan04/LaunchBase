package httpapi

import (
	"context"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/security"
)

type opsStatus struct {
	Status            string `json:"status"`
	QueueDepth        int64  `json:"queueDepth"`
	ReadyDeployments  int    `json:"readyDeployments"`
	Building          int    `json:"building"`
	FailedDeployments int    `json:"failedDeployments"`
	DiskPath          string `json:"diskPath,omitempty"`
	DiskUsedBytes     uint64 `json:"diskUsedBytes,omitempty"`
	DiskFreeBytes     uint64 `json:"diskFreeBytes,omitempty"`
	DiskTotalBytes    uint64 `json:"diskTotalBytes,omitempty"`
	CheckedAt         string `json:"checkedAt"`
}

func (h handlers) opsStatus(w http.ResponseWriter, r *http.Request) {
	want := strings.TrimSpace(os.Getenv("OPS_TOKEN"))
	if want == "" {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	got := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(got, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(got, "Bearer ")) != want {
		writeError(w, http.StatusUnauthorized, "unauthorized", "ops token required")
		return
	}
	if h.svc == nil || h.svc.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "store unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	queueDepth, err := h.svc.Store.CountPendingOutbox(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "could not read queue")
		return
	}
	ready, err := h.svc.Store.CountDeploymentsByStatus(ctx, "", security.ReadyStatuses)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "could not read deployments")
		return
	}
	building, err := h.svc.Store.CountDeploymentsByStatus(ctx, "", security.BuildStatuses)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "could not read builds")
		return
	}
	failed, err := h.svc.Store.CountDeploymentsByStatus(ctx, "", []string{"FAILED"})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "not_ready", "could not read failures")
		return
	}

	out := opsStatus{
		Status:            "ok",
		QueueDepth:        queueDepth,
		ReadyDeployments:  ready,
		Building:          building,
		FailedDeployments: failed,
		CheckedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	diskPath := strings.TrimSpace(os.Getenv("OPS_DISK_PATH"))
	if diskPath == "" {
		diskPath = "/"
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(diskPath, &st); err == nil {
		total := st.Blocks * uint64(st.Bsize)
		free := st.Bavail * uint64(st.Bsize)
		out.DiskPath = diskPath
		out.DiskTotalBytes = total
		out.DiskFreeBytes = free
		out.DiskUsedBytes = total - free
	}
	writeJSON(w, http.StatusOK, out)
}
