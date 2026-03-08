package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"cargo/internal/project"
)

// projectInfo is the JSON representation of a project in list responses.
type projectInfo struct {
	Name         string `json:"name"`
	GitURL       string `json:"git_url"`
	Revision     string `json:"revision"`
	PollInterval string `json:"poll_interval,omitempty"`
}

// syncResultResponse is the JSON representation of a SyncResult.
type syncResultResponse struct {
	ProjectName string `json:"project_name"`
	Commit      string `json:"commit,omitempty"`
	Error       string `json:"error,omitempty"`
}

func toSyncResultResponse(r project.SyncResult) syncResultResponse {
	resp := syncResultResponse{
		ProjectName: r.ProjectName,
		Commit:      r.Commit,
	}
	if r.Error != nil {
		resp.Error = r.Error.Error()
	}
	return resp
}

// ListProjects handles GET /api/v1/projects and returns all configured projects.
func ListProjects(mgr *project.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		projects := make([]projectInfo, 0, len(mgr.Config.Projects))
		for _, p := range mgr.Config.Projects {
			projects = append(projects, projectInfo{
				Name:         p.Name,
				GitURL:       p.Git.URL,
				Revision:     p.Git.Revision,
				PollInterval: p.PollInterval,
			})
		}
		c.JSON(http.StatusOK, gin.H{"projects": projects})
	}
}

// SyncProject handles POST /api/v1/projects/:name/sync and syncs a single project.
func SyncProject(mgr *project.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		result := mgr.SyncProject(name)
		resp := toSyncResultResponse(result)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, resp)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// SyncAllProjects handles POST /api/v1/projects/sync and syncs all projects.
func SyncAllProjects(mgr *project.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		results := mgr.SyncAll()
		responses := make([]syncResultResponse, 0, len(results))
		hasError := false
		for _, r := range results {
			resp := toSyncResultResponse(r)
			if r.Error != nil {
				hasError = true
			}
			responses = append(responses, resp)
		}

		statusCode := http.StatusOK
		if hasError {
			statusCode = http.StatusMultiStatus
		}
		c.JSON(statusCode, gin.H{"results": responses})
	}
}

// GetProjectStatus handles GET /api/v1/projects/:name/status and returns compose ps output.
func GetProjectStatus(mgr *project.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		status, err := mgr.StatusProject(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"project": name,
			"status":  status,
		})
	}
}
