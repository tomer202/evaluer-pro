package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const openAPISpecJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Evaluer REST API",
    "description": "Student evaluation, grade calculation, and recommendation API for Hive LMS platform.",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "http://localhost:8000",
      "description": "Local Development Server"
    }
  ],
  "paths": {
    "/api/v1/config": {
      "get": {
        "summary": "Get active course configuration",
        "responses": { "200": { "description": "Active CourseConfig JSON" } }
      },
      "put": {
        "summary": "Full replacement of course configuration",
        "requestBody": {
          "required": true,
          "content": { "application/json": { "schema": { "type": "object" } } }
        },
        "responses": { "200": { "description": "Updated CourseConfig JSON" } }
      },
      "patch": {
        "summary": "Partial update of course weights or exercise scores",
        "requestBody": {
          "required": true,
          "content": { "application/json": { "schema": { "type": "object" } } }
        },
        "responses": { "200": { "description": "Updated CourseConfig JSON" } }
      }
    },
    "/api/v1/config/generate": {
      "post": {
        "summary": "Generate course configuration skeleton from Hive DB",
        "parameters": [
          { "name": "save", "in": "query", "schema": { "type": "boolean", "default": true } },
          { "name": "path", "in": "query", "schema": { "type": "string" } }
        ],
        "responses": { "200": { "description": "Generated config response" } }
      }
    },
    "/api/v1/config/generate/preview": {
      "post": {
        "summary": "Preview course configuration skeleton in RAM",
        "responses": { "200": { "description": "Preview CourseConfig JSON" } }
      }
    },
    "/api/v1/calc/exercise": {
      "post": {
        "summary": "Calculate grades for a specific exercise",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "subject_name": "Sukmi",
                "module_name": "test",
                "exercise_name": "today",
                "students": ["gal"]
              }
            }
          }
        },
        "responses": { "200": { "description": "Exercise calculation grades" } }
      }
    },
    "/api/v1/calc/module": {
      "post": {
        "summary": "Calculate grades for a specific module",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "subject_name": "Sukmi",
                "module_name": "test",
                "students": ["gal"]
              }
            }
          }
        },
        "responses": { "200": { "description": "Module calculation grades" } }
      }
    },
    "/api/v1/calc/subject": {
      "post": {
        "summary": "Calculate grades for a specific subject",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "subject_name": "Sukmi",
                "students": ["gal"]
              }
            }
          }
        },
        "responses": { "200": { "description": "Subject calculation grades" } }
      }
    },
    "/api/v1/calc/course": {
      "post": {
        "summary": "Calculate overall course grades",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "students": []
              }
            }
          }
        },
        "responses": { "200": { "description": "Course calculation grades for students" } }
      }
    },
    "/api/v1/calc/course/status": {
      "post": {
        "summary": "Calculate status summary breakdown",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "students": []
              }
            }
          }
        },
        "responses": { "200": { "description": "Course status breakdown analytics" } }
      }
    },
    "/api/v1/calc/student": {
      "post": {
        "summary": "Full student grade hierarchy breakdown",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "username": "gal"
              }
            }
          }
        },
        "responses": { "200": { "description": "Detailed student grade tree" } }
      }
    },
    "/api/v1/calc/student/recommendations": {
      "post": {
        "summary": "Student sequential recommendation queue",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {
                "username": "gal",
                "target_score": 70.0
              }
            }
          }
        },
        "responses": { "200": { "description": "Prioritized recommendation queue per subject" } }
      }
    }
  }
}`

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Evaluer REST API - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin:0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" charset="UTF-8"></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" charset="UTF-8"></script>
  <script>
    window.onload = function() {
      window.ui = SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        plugins: [
          SwaggerUIBundle.plugins.DownloadUrl
        ],
        layout: "StandaloneLayout"
      });
    };
  </script>
</body>
</html>`

func RegisterSwaggerRoutes(r *gin.Engine) {
	r.GET("/openapi.json", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, openAPISpecJSON)
	})

	docsHandler := func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, swaggerUIHTML)
	}

	r.GET("/docs", docsHandler)
	r.GET("/docs/*any", docsHandler)
	r.GET("/swagger", docsHandler)
	r.GET("/swagger/*any", docsHandler)
}
