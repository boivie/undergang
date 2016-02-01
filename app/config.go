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

type PathInfo struct {
	Prefix          string `json:"prefix"`
	SSHTunnel       *SSHTunnel `json:"ssh_tunnel"`

	Backend         *Backend `json:"backend"`
	StaticOverrides map[string]string `json:"static_overrides"`
}
