package app
import "golang.org/x/crypto/ssh"

func runBootstrap(ssh *ssh.Client, info PathInfo, progress chan <- ProgressCmd) {
	type BootstrapStep struct {
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	status := struct {
		Steps []BootstrapStep `json:"steps"`
	}{make([]BootstrapStep, 0)}

	for _, cmd := range info.SSHTunnel.Bootstrap {
		status.Steps = append(status.Steps, BootstrapStep{cmd.Description, ""})
	}

	for idx, cmd := range info.SSHTunnel.Bootstrap {
		status.Steps[idx].Status = "started"
		progress <- ProgressCmd{"bootstrap_status", status}
		session, _ := ssh.NewSession()
		defer session.Close()
		session.Run(cmd.Command)
		status.Steps[idx].Status = "done"
		progress <- ProgressCmd{"bootstrap_status", status}
	}
}