package api

import (
	"net/http"

	"evaluer/internal/config"
	"evaluer/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(calc *service.GradeCalculator, configMgr *config.ConfigManager) *gin.Engine {
	r := gin.Default()

	// CORS Setup & Options preflight handler
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Accept"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	handler := NewAPIHandler(calc, configMgr)

	// Swagger UI & OpenAPI spec
	RegisterSwaggerRoutes(r)

	v1 := r.Group("/api/v1")
	{
		// Config Routes
		configGroup := v1.Group("/config")
		{
			configGroup.GET("", handler.GetConfig)
			configGroup.POST("/generate", handler.GenerateConfig)
			configGroup.POST("/generate/preview", handler.PreviewConfig)
			configGroup.PUT("", handler.PutConfig)
			configGroup.PATCH("", handler.PatchConfig)
			configGroup.OPTIONS("", func(c *gin.Context) { c.Status(http.StatusOK) })
		}

		// Calc Routes
		calcGroup := v1.Group("/calc")
		{
			calcGroup.POST("/exercise", handler.CalcExercise)
			calcGroup.POST("/module", handler.CalcModule)
			calcGroup.POST("/subject", handler.CalcSubject)
			calcGroup.POST("/course", handler.CalcCourse)
			calcGroup.POST("/course/status", handler.CalcCourseStatus)
			calcGroup.POST("/student", handler.CalcStudent)
			calcGroup.POST("/student/recommendations", handler.CalcStudentRecommendations)

			// Preflight OPTIONS for all calc routes
			calcGroup.OPTIONS("/*path", func(c *gin.Context) { c.Status(http.StatusOK) })
		}
	}

	return r
}
