package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RateLimitResult holds the result of a rate limit check
type RateLimitResult struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetIn   time.Duration
}

// RateLimitChecker defines a function that checks whether a request is allowed
type RateLimitChecker func(key string) (*RateLimitResult, error)

// RateLimitMiddleware creates a middleware that applies rate limiting on auth endpoints.
// The checker function receives the client IP and returns a rate limit result.
func RateLimitMiddleware(checker RateLimitChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !isAuthEndpoint(path) {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		result, err := checker(clientIP)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "rate limit error",
				"code":    "INTERNAL_ERROR",
			})
			c.Abort()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "rate limit exceeded",
				"code":    "RATE_LIMITED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// MessageRateLimitMiddleware creates a middleware for message rate limiting.
// Requires user_id to be set in gin context by AuthMiddleware.
func MessageRateLimitMiddleware(checker RateLimitChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.Next()
			return
		}

		result, err := checker(userID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "rate limit error",
				"code":    "INTERNAL_ERROR",
			})
			c.Abort()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "message rate limit exceeded",
				"code":    "RATE_LIMITED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CallRateLimitMiddleware creates a middleware for call rate limiting.
// Requires user_id to be set in gin context by AuthMiddleware.
func CallRateLimitMiddleware(checker RateLimitChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		userID, ok := userIDVal.(uuid.UUID)
		if !ok {
			c.Next()
			return
		}

		result, err := checker(userID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "rate limit error",
				"code":    "INTERNAL_ERROR",
			})
			c.Abort()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "call rate limit exceeded",
				"code":    "RATE_LIMITED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// setRateLimitHeaders sets standard rate limit response headers
func setRateLimitHeaders(c *gin.Context, result *RateLimitResult) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(int64(result.ResetIn.Seconds()), 10))
}

// isAuthEndpoint checks if the request path is an auth endpoint
func isAuthEndpoint(path string) bool {
	authPaths := []string{
		"/v1/auth/login",
		"/v1/auth/register",
		"/v1/auth/refresh",
		"/v1/auth/password/forgot",
		"/v1/auth/password/reset",
	}
	for _, p := range authPaths {
		if path == p {
			return true
		}
	}
	return false
}
