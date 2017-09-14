package app

type configCommand struct {
	Description string `json:"description"`
	Command     string `json:"command"`
}

type configSSHTunnel struct {
	Address        string `json:"address"`
	Username       string `json:"username"`
	SSHKeyContents string `json:"ssh_key_contents"`
	SSHKeyFileName string `json:"ssh_key_filename"`

	Bootstrap []configCommand `json:"bootstrap"`
	Run       *configCommand  `json:"run"`
}

type configBackend struct {
	Address  string `json:"address"`
	BasePath string `json:"base_path"`
}

type configProvisioning struct {
	// If this is 'started', undergang will periodically poll the /path endpoint every 5 seconds
	// until it is 'done', 'failed' or the Provisioning field is missing.
	Status string `json:"status"`
}

type configBasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Similar to an oauth2 authentication flow. If the user isn't authenticated,
// we redirect him to AuthUrl?redirect_uri=$original_url/__ug_auth. When authenticated, the user will be
// redirected back to $original_url/__ug_auth?code=$code. UG will then do a HTTP POST to ValidateUrl
// with the code, and if correct, it will return 200 and UG will set a cookie limited to the domain
// and redirect the user back to $original_url.
type configServerAuth struct {
	AuthURL     string `json:"auth_url"`
	ValidateURL string `json:"validate_url"`
}

type configStyle struct {
	BackgroundColor string `json:"background_color"`
}

type configProgressPage struct {
	Style    *configStyle `json:"style"`
	Filename string       `json:"filename"`
	URL      string       `json:"url"`
	Hostname string       `json:"hostname"`
}

// PathInfo represents the configuration of a backend
type PathInfo struct {
	Host         string              `json:"host"`
	Prefix       string              `json:"prefix"`
	Provisioning *configProvisioning `json:"provisioning"`
	SSHTunnel    *configSSHTunnel    `json:"ssh_tunnel"`

	Backend         *configBackend    `json:"backend"`
	StaticOverrides map[string]string `json:"static_overrides"`

	ProgressPage *configProgressPage `json:"progress_page"`

	BasicAuth *configBasicAuth `json:"basic_auth"`

	ServerAuth *configServerAuth `json:"server_auth"`
}
