package admin

// User is the administrative view, including fields that must never appear in
// public API documentation.
type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	IsBanned      bool   `json:"is_banned"`
	InternalNotes string `json:"internal_notes"`
}
