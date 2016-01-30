package app

type ServerInfo struct {
	Address   string `json:"address"`
	Username  string `json:"username"`
	SSHKey    string `json:"ssh_key"`
	Bootstrap []string `json:"bootstrap"`
	Run       string `json:"run"`
}

type HttpProxy struct {
	Address  string `json:"address"`
	BasePath string `json:"base_path"`
}

type PathInfo struct {
	Prefix          string `json:"prefix"`
	Server          ServerInfo `json:"server"`
	HttpProxy       *HttpProxy `json:"http_proxy"`
	StaticOverrides map[string]string `json:"static_overrides"`
}
