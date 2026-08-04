// Package main covers gin. Two things differ from chi and both are currently
// unhandled:
//
//   - gin writes path parameters as :id, which must be normalised to {id} for
//     OpenAPI and surfaced as a path parameter.
//   - the dominant gin idiom assigns a group to a variable
//     (v1 := r.Group("/v1")), which requires tracking a router through an
//     assignment.
package main

import (
	"net/http"

	"github.com/Zachacious/go-respec/respec"
	"github.com/gin-gonic/gin"
)

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CreateUserRequest struct {
	Name string `json:"name"`
}

// GetUser returns a single user.
func GetUser(c *gin.Context) { c.JSON(http.StatusOK, User{}) }

// ListUsers returns all users.
func ListUsers(c *gin.Context) { c.JSON(http.StatusOK, []User{}) }

// CreateUser adds a user.
func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	c.ShouldBindJSON(&req)
	c.JSON(http.StatusCreated, User{})
}

func main() {
	r := gin.Default()

	// Registered straight on the engine, with a gin-style path parameter.
	r.GET("/users/:id", respec.Handler(GetUser).Summary("Get a user").Unwrap())

	// Registered on a group held in a variable.
	v1 := r.Group("/v1")
	v1.GET("/users", respec.Handler(ListUsers).Summary("List users").Unwrap())
	v1.POST("/users", respec.Handler(CreateUser).Summary("Create a user").Unwrap())

	r.Run(":8080")
}
