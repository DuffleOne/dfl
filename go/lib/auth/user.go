package auth

type AuthUser struct {
	ID       string
	Username string
	Scopes   string
}

func (au AuthUser) Can(action string) bool {
	return Can(action, au.Scopes)
}
