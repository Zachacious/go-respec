// Package main covers echo, including the group-in-a-variable idiom and echo's
// colon path parameters.
package main

import (
	"net/http"

	"github.com/Zachacious/go-respec/respec"
	"github.com/labstack/echo/v4"
)

type Product struct {
	SKU   string  `json:"sku"`
	Price float64 `json:"price"`
}

type NewProductRequest struct {
	SKU string `json:"sku"`
}

// GetProduct returns one product.
func GetProduct(c echo.Context) error {
	return c.JSON(http.StatusOK, Product{})
}

// ListProducts returns every product.
func ListProducts(c echo.Context) error {
	return c.JSON(http.StatusOK, []Product{})
}

// CreateProduct adds a product.
func CreateProduct(c echo.Context) error {
	var req NewProductRequest
	c.Bind(&req)
	return c.JSON(http.StatusCreated, Product{})
}

func main() {
	e := echo.New()

	e.GET("/products/:sku", respec.Handler(GetProduct).Summary("Get a product").Unwrap())

	v1 := e.Group("/v1")
	v1.GET("/products", respec.Handler(ListProducts).Summary("List products").Unwrap())
	v1.POST("/products", respec.Handler(CreateProduct).Summary("Create a product").Unwrap())

	e.Logger.Fatal(e.Start(":8080"))
}
