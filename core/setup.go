package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// NOTE: the Feishu/Lark and Weixin (ilink) QR-provisioning setup endpoints
// (handleSetupFeishuBegin/Poll/Save, handleSetupWeixinBegin/Poll/Save and the
// matching request types) were removed together with those platforms. Only the
// generic manual platform-add path remains.

// ── Generic platform add (manual config) ─────────────────────

type AddPlatformRequest struct {
	Type      string         `json:"type"`
	Options   map[string]any `json:"options"`
	WorkDir   string         `json:"work_dir"`
	AgentType string         `json:"agent_type"`
}

func (m *ManagementServer) handleProjectAddPlatform(w http.ResponseWriter, r *http.Request, projectName string) {
	if r.Method != http.MethodPost {
		mgmtError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}
	var req AddPlatformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		mgmtError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Type == "" {
		mgmtError(w, http.StatusBadRequest, "type is required")
		return
	}
	workDir, err := validateProjectWorkDir(req.WorkDir)
	if err != nil {
		mgmtError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.WorkDir = workDir
	if m.addPlatformToProject == nil {
		mgmtError(w, http.StatusServiceUnavailable, "config persistence not available")
		return
	}
	if err := m.addPlatformToProject(projectName, req.Type, req.Options, req.WorkDir, req.AgentType); err != nil {
		mgmtError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}
	mgmtJSON(w, http.StatusCreated, map[string]any{
		"message":          fmt.Sprintf("platform %q added to project %q", req.Type, projectName),
		"restart_required": true,
	})
}

func validateProjectWorkDir(workDir string) (string, error) {
	trimmed := strings.TrimSpace(workDir)
	if trimmed == "" {
		return "", nil
	}

	info, err := os.Stat(trimmed)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("work_dir does not exist: %s", trimmed)
		}
		return "", fmt.Errorf("work_dir is not accessible: %s: %w", trimmed, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("work_dir is not a directory: %s", trimmed)
	}
	return trimmed, nil
}
