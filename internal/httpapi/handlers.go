package httpapi

import (
	"net/http"

	"github.com/soulteary/gorge-diff/internal/engine"

	"github.com/labstack/echo/v4"
)

type Deps struct {
	Token string
}

type apiResponse struct {
	Data  any       `json:"data,omitempty"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func RegisterRoutes(e *echo.Echo, deps *Deps) {
	e.GET("/", healthPing())
	e.GET("/healthz", healthPing())

	g := e.Group("/api/diff")
	g.Use(tokenAuth(deps))

	g.POST("/generate", generateDiff())
	g.POST("/prose", proseDiff())
}

func tokenAuth(deps *Deps) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if deps.Token == "" {
				return next(c)
			}
			token := c.Request().Header.Get("X-Service-Token")
			if token == "" {
				token = c.QueryParam("token")
			}
			if token == "" || token != deps.Token {
				return c.JSON(http.StatusUnauthorized, &apiResponse{
					Error: &apiError{Code: "ERR_UNAUTHORIZED", Message: "missing or invalid service token"},
				})
			}
			return next(c)
		}
	}
}

func healthPing() echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func respondOK(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, &apiResponse{Data: data})
}

func respondErr(c echo.Context, status int, code, msg string) error {
	return c.JSON(status, &apiResponse{
		Error: &apiError{Code: code, Message: msg},
	})
}

func generateDiff() echo.HandlerFunc {
	return func(c echo.Context) error {
		var req engine.DiffRequest
		if err := c.Bind(&req); err != nil {
			return respondErr(c, http.StatusBadRequest, "ERR_BAD_REQUEST", err.Error())
		}

		result := engine.GenerateUnifiedDiff(&req)
		return respondOK(c, result)
	}
}

func proseDiff() echo.HandlerFunc {
	return func(c echo.Context) error {
		var req engine.ProseRequest
		if err := c.Bind(&req); err != nil {
			return respondErr(c, http.StatusBadRequest, "ERR_BAD_REQUEST", err.Error())
		}

		result := engine.GenerateProseDiff(&req)
		return respondOK(c, result)
	}
}
