package digitalocean

import "golang.org/x/oauth2"

type tokenSource struct {
	AccessToken string
}

// Token method is needed to make the token of type oauth2
func (t *tokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}

	return token, nil
}
