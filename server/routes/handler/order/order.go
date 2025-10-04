package order

import (
	"github.com/labstack/echo/v4"
)

var router *echo.Echo

func InitOrderRoutes(e *echo.Echo) {
	router = e
	orderRoutes()
}

func orderRoutes() {
	orderGroup := router.Group("/order")
	{
		orderGroup.POST("/buy", placeBuyOrder)
		orderGroup.POST("/sell", placeSellOrder)
		orderGroup.POST("/cancel", cancelOrder)
	}
}

func placeBuyOrder(c echo.Context) error {
	return c.String(200, "Buy order placed")
}

func placeSellOrder(c echo.Context) error {
	return c.String(200, "Sell order placed")
}

func cancelOrder(c echo.Context) error {
	return c.String(200, "Order cancelled")
}
