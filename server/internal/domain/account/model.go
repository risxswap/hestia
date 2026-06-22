package account

type User struct {
	ID               int64  `db:"id"`
	PublicID         string `db:"public_id"`
	WechatOpenID     string `db:"wechat_openid"`
	Nickname         string `db:"nickname"`
	OnboardingStatus string `db:"onboarding_status"`
	Status           string `db:"status"`
}

type DevUserInput struct {
	PublicID     string
	WechatOpenID string
	Nickname     string
}

type DevLoginInput struct {
	Nickname string `json:"nickname"`
	DevKey   string `json:"dev_key"`
}

type DevLoginResult struct {
	UserPublicID     string `json:"user_public_id"`
	Token            string `json:"token"`
	OnboardingStatus string `json:"onboarding_status"`
}
