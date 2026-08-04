package internal

// User is a third view sharing the same type name. With three colliding types,
// none may keep the unqualified "User" component name - otherwise one of them
// reads as the canonical one.
type User struct {
	ShadowBan bool `json:"shadow_ban"`
}
