package auth

import "github.com/gin-gonic/gin"

const userContextKey = "hestia.user"

type User struct {
	UserID       int64
	UserPublicID string
	Surface      string
}

func SetUserContext(c *gin.Context, user User) {
	c.Set(userContextKey, user)
}

func UserFromContext(c *gin.Context) (User, bool) {
	value, ok := c.Get(userContextKey)
	if !ok {
		return User{}, false
	}
	user, ok := value.(User)
	return user, ok
}
