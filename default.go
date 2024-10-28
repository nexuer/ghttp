package ghttp

var defaultClient = NewClient()

func SetClient(c *Client) {
	defaultClient = c
}
