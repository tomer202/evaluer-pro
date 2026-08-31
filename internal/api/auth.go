package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecretKey = []byte("evaluer-secret-jwt-key-2026-secure")

type UserCredentials struct {
	Username string
	Password string
	Role     string
}

var userStore = map[string]UserCredentials{
	"admin": {
		Username: "admin",
		Password: "QWEqwe123",
		Role:     "admin",
	},
	"chief": {
		Username: "chief",
		Password: "EWQewq321",
		Role:     "chief",
	},
	"cheif": {
		Username: "chief",
		Password: "EWQewq321",
		Role:     "chief",
	},
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int64  `json:"expires_in"`
	Username  string `json:"username"`
	Role      string `json:"role"`
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateJWT(username, role string) (string, int64, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	expiresInSeconds := int64(24 * 3600)
	claims := &Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "evaluer-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	return tokenString, expiresInSeconds, err
}

func ValidateJWT(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecretKey, nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token claims")
}

func (h *APIHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	u, ok := userStore[req.Username]
	if !ok || u.Password != req.Password {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	tokenStr, expiresIn, err := GenerateJWT(u.Username, u.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate authentication token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token:     tokenStr,
		TokenType: "Bearer",
		ExpiresIn: expiresIn,
		Username:  u.Username,
		Role:      u.Role,
	})
}

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Set("authenticated", false)
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
			return
		}

		claims, err := ValidateJWT(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("authenticated", true)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func checkCalcPermission(c *gin.Context, students []string) error {
	authenticated, _ := c.Get("authenticated")
	isAuthenticated, _ := authenticated.(bool)
	if !isAuthenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required for calculation route"})
		return fmt.Errorf("unauthenticated")
	}

	role, _ := c.Get("role")
	userRole, _ := role.(string)

	if userRole == "chief" {
		return nil
	}

	if userRole == "admin" {
		if len(students) < 1 || len(students) > 5 {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Admin user is restricted to specifying between 1 and 5 target students",
			})
			return fmt.Errorf("forbidden: admin student count limit")
		}
		return nil
	}

	c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: insufficient permissions"})
	return fmt.Errorf("forbidden")
}

func checkCoursePermission(c *gin.Context) error {
	authenticated, _ := c.Get("authenticated")
	isAuthenticated, _ := authenticated.(bool)
	if !isAuthenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required for course calculation"})
		return fmt.Errorf("unauthenticated")
	}

	role, _ := c.Get("role")
	userRole, _ := role.(string)

	if userRole != "chief" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Course calculation is allowed only for chief role",
		})
		return fmt.Errorf("forbidden: chief role required")
	}

	return nil
}

func checkCourseSummaryPermission(c *gin.Context) error {
	authenticated, _ := c.Get("authenticated")
	isAuthenticated, _ := authenticated.(bool)
	if !isAuthenticated {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required for course summary calculation"})
		return fmt.Errorf("unauthenticated")
	}
	return nil
}
