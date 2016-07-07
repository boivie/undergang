package app

type Command struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

type SSHTunnel struct {
	Address        string `json:"address"`
	Username       string `json:"username"`
	SSHKeyContents string `json:"ssh_key_contents"`
	SSHKeyFileName string `json:"ssh_key_filename"`

	Bootstrap []Command `json:"bootstrap"`
	Run       *Command  `json:"run"`
}

type Backend struct {
	Address  string `json:"address"`
	BasePath string `json:"base_path"`
}

type Provisioning struct {
	// If this is 'started', undergang will periodically poll the /path endpoint every 5 seconds
	// until it is 'done', 'failed' or the Provisioning field is missing.
	Status string `json:"status"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Similar to an oauth2 authentication flow. If the user isn't authenticated,
// we redirect him to AuthUrl?redirect_uri=$original_url/__ug_auth. When authenticated, the user will be
// redirected back to $original_url/__ug_auth?code=$code. UG will then do a HTTP POST to ValidateUrl
// with the code, and if correct, it will return 200 and UG will set a cookie limited to the domain
// and redirect the user back to $original_url.
type ServerAuth struct {
	AuthUrl     string `json:"auth_url"`
	ValidateUrl string `json:"validate_url"`
}

type Style struct {
	BackgroundColor string `json:"background_color"`
}

type ProgressPage struct {
	Style    *Style `json:"style"`
	Filename string `json:"filename"`
	Url      string `json:"url"`
}

type PathInfo struct {
	Host         string        `json:"host"`
	Prefix       string        `json:"prefix"`
	Provisioning *Provisioning `json:"provisioning"`
	SSHTunnel    *SSHTunnel    `json:"ssh_tunnel"`

	Backend         *Backend          `json:"backend"`
	StaticOverrides map[string]string `json:"static_overrides"`

	ProgressPage *ProgressPage `json:"progress_page"`

	BasicAuth *BasicAuth `json:"basic_auth"`

	ServerAuth *ServerAuth `json:"server_auth"`
}
