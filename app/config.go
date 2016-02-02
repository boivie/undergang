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

	Bootstrap      []Command `json:"bootstrap"`
	Run            *Command `json:"run"`
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

type PathInfo struct {
	Host            string `json:"host"`
	Prefix          string `json:"prefix"`
	Provisioning    *Provisioning `json:"provisioning"`
	SSHTunnel       *SSHTunnel `json:"ssh_tunnel"`

	Backend         *Backend `json:"backend"`
	StaticOverrides map[string]string `json:"static_overrides"`

	BasicAuth       *BasicAuth `json:"basic_auth"`
}
