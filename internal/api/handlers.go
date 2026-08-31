package api

import (
	"encoding/json"
	"net/http"
	"time"

	"evaluer/internal/config"
	"evaluer/internal/model"
	"evaluer/internal/service"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	calc      *service.GradeCalculator
	configMgr *config.ConfigManager
}

func NewAPIHandler(calc *service.GradeCalculator, configMgr *config.ConfigManager) *APIHandler {
	return &APIHandler{
		calc:      calc,
		configMgr: configMgr,
	}
}

// Health Handler

func (h *APIHandler) HealthCheck(c *gin.Context) {
	if h.calc != nil && h.calc.IsHiveAlive() {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"hive":      "alive",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "unhealthy",
			"hive":      "unreachable",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// Config Handlers

func (h *APIHandler) GetConfig(c *gin.Context) {
	jsonBytes, err := h.configMgr.GetActiveConfigRawJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", jsonBytes)
}

func (h *APIHandler) GenerateConfig(c *gin.Context) {
	saveQuery := c.DefaultQuery("save", "true")
	pathQuery := c.Query("path")

	if saveQuery == "true" {
		jsonBytes, err := h.configMgr.GenerateAndSaveRaw(pathQuery)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":     "Config successfully generated and saved to disk",
			"config_path": h.configMgr.GetConfigPath(),
			"config":      json.RawMessage(jsonBytes),
		})
	} else {
		jsonBytes, err := h.configMgr.GenerateSkeletonRawJSON()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", jsonBytes)
	}
}

func (h *APIHandler) PreviewConfig(c *gin.Context) {
	jsonBytes, err := h.configMgr.GenerateSkeletonRawJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", jsonBytes)
}

func (h *APIHandler) PutConfig(c *gin.Context) {
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pathQuery := c.Query("path")
	if err := h.configMgr.SaveRawJSON(pathQuery, bodyBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	jsonBytes, err := h.configMgr.GetActiveConfigRawJSON()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", jsonBytes)
}

func (h *APIHandler) PatchConfig(c *gin.Context) {
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jsonBytes, err := h.configMgr.PatchRawJSON(bodyBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", jsonBytes)
}

// Calc Handlers

func (h *APIHandler) CalcExercise(c *gin.Context) {
	var req model.ExerciseCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := checkCalcPermission(c, req.Students); err != nil {
		return
	}

	res, err := h.calc.CalculateExercise(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcModule(c *gin.Context) {
	var req model.ModuleCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := checkCalcPermission(c, req.Students); err != nil {
		return
	}

	res, err := h.calc.CalculateModule(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcSubject(c *gin.Context) {
	var req model.SubjectCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := checkCalcPermission(c, req.Students); err != nil {
		return
	}

	res, err := h.calc.CalculateSubject(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcCourse(c *gin.Context) {
	var req model.CourseCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := checkCoursePermission(c); err != nil {
		return
	}

	res, err := h.calc.CalculateCourse(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcCourseSummary(c *gin.Context) {
	var req model.CourseCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := checkCourseSummaryPermission(c); err != nil {
		return
	}

	res, err := h.calc.CalculateCourseSummary(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcCourseStatus(c *gin.Context) {
	var req model.StatusCalcRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.calc.CalculateStatusBreakdown(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcStudent(c *gin.Context) {
	var req model.StudentDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.calc.CalculateStudent(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) CalcStudentRecommendations(c *gin.Context) {
	var req model.StudentRecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.calc.CalculateRecommendations(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *APIHandler) GetStudents(c *gin.Context) {
	students, err := h.calc.GetStudents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, students)
}
