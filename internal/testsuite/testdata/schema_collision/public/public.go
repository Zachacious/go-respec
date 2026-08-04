package public

// User is the publicly visible view. It deliberately shares its name with
// admin.User.
type User struct {
	Handle string `json:"handle"`
}
