package middleware

import (
	"net/http"
	"strconv"

	"sentinal-chat/internal/redis"
	"sentinal-chat/internal/services"
	"sentinal-chat/internal/transport/httpdto"

	"github.com/gin-gonic/gin"
)

// RateLimitMiddleware creates a middleware that applies rate limiting
// Uses the RateLimiter from the redis package
func RateLimitMiddleware(limiter *redis.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP for auth rate limiting
		clientIP := c.ClientIP()

		// Check if this is an auth endpoint
		path := c.Request.URL.Path
		if isAuthEndpoint(path) {
			result, err := limiter.AllowAuth(c.Request.Context(), clientIP)
			if err != nil {
				c.JSON(http.StatusInternalServerError, httpdto.NewErrorResponse("rate limit error", "INTERNAL_ERROR"))
				c.Abort()
				return
			}

			// Set rate limit headers
			setRateLimitHeaders(c, result)

			if !result.Allowed {
				c.JSON(http.StatusTooManyRequests, httpdto.NewErrorResponse("rate limit exceeded", "RATE_LIMITED"))
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// MessageRateLimitMiddleware creates a middleware for message rate limiting
// Should be applied to message endpoints after auth middleware
func MessageRateLimitMiddleware(limiter *redis.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := services.UserIDFromContext(c.Request.Context())
		if !ok {
			// No user context, skip rate limiting (auth middleware will handle)
			c.Next()
			return
		}

		result, err := limiter.AllowMessage(c.Request.Context(), userID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, httpdto.NewErrorResponse("rate limit error", "INTERNAL_ERROR"))
			c.Abort()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, httpdto.NewErrorResponse("message rate limit exceeded", "RATE_LIMITED"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// CallRateLimitMiddleware creates a middleware for call rate limiting
// Should be applied to call initiation endpoints after auth middleware
func CallRateLimitMiddleware(limiter *redis.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := services.UserIDFromContext(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		result, err := limiter.AllowCall(c.Request.Context(), userID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, httpdto.NewErrorResponse("rate limit error", "INTERNAL_ERROR"))
			c.Abort()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, httpdto.NewErrorResponse("call rate limit exceeded", "RATE_LIMITED"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// WebSocketRateLimitMiddleware creates a middleware for WebSocket connection rate limiting
func WebSocketRateLimitMiddleware(limiter *redis.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := services.UserIDFromContext(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		result, err := limiter.AllowWebSocket(c.Request.Context(), userID.String())
		if err != nil {
			c.JSON(http.StatusInternalServerError, httpdto.NewErrorResponse("rate limit error", "INTERNAL_ERROR"))
			c.Abort()
			return
		}

		setRateLimitHeaders(c, result)

		if !result.Allowed {
			c.JSON(http.StatusTooManyRequests, httpdto.NewErrorResponse("connection rate limit exceeded", "RATE_LIMITED"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// setRateLimitHeaders sets standard rate limit response headers
func setRateLimitHeaders(c *gin.Context, result *redis.RateLimitResult) {
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
